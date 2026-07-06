// Made by YTSworks
// YTS工作室製作
package backup

import (
	"archive/zip"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"management-server/config"
	"management-server/services/dbencryption"
	"management-server/services/snmp"
)

type Service struct {
	db     *sql.DB
	config *config.Config
	snmp   *snmp.Collector
}

func NewService(db *sql.DB, cfg *config.Config, collector *snmp.Collector) *Service {
	return &Service{db: db, config: cfg, snmp: collector}
}

func (s *Service) ExportBackupArchive() (ArchiveResult, error) {
	var busy, walPages, checkpointedPages int
	row := s.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)")
	if err := row.Scan(&busy, &walPages, &checkpointedPages); err != nil {
		return ArchiveResult{}, err
	}

	dbPath := s.config.Database.Path
	walPath := dbPath + "-wal"
	if info, err := os.Stat(walPath); err == nil && info.Size() > 0 {
		return ArchiveResult{}, ErrBackupCheckpointIncomplete
	}

	tempFile, err := os.CreateTemp("", "nms_backup_*.zip")
	if err != nil {
		return ArchiveResult{}, err
	}
	defer tempFile.Close()

	zipWriter := zip.NewWriter(tempFile)
	addFile := func(src, dst string) error {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return nil
		}

		file, err := os.Open(src)
		if err != nil {
			return err
		}
		defer file.Close()

		writer, err := zipWriter.Create(dst)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	}

	if err := addFile(dbPath, "nms.db"); err != nil {
		return ArchiveResult{}, err
	}
	if err := addFile(dbPath+"-wal", "nms.db-wal"); err != nil {
		return ArchiveResult{}, err
	}
	if err := addFile(dbPath+"-shm", "nms.db-shm"); err != nil {
		return ArchiveResult{}, err
	}

	configCandidates := []string{"data/config.yaml", "config.yaml"}
	for _, candidate := range configCandidates {
		if err := addFile(candidate, "config.yaml"); err == nil {
			break
		}
	}

	uploadsPath := "data/uploads"
	_ = filepath.Walk(uploadsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}

		relPath, relErr := filepath.Rel("data", path)
		if relErr != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		return addFile(path, relPath)
	})

	if err := zipWriter.Close(); err != nil {
		return ArchiveResult{}, err
	}

	prefix := "nms"
	switch runtime.GOOS {
	case "windows":
		prefix = "win_nms"
	case "linux":
		prefix = "linux_nms"
	}

	filename := fmt.Sprintf("%s_backup_%s.zip", prefix, time.Now().Format("20060102_150405"))
	return ArchiveResult{
		Path:        tempFile.Name(),
		Filename:    filename,
		ContentType: "application/zip",
	}, nil
}

func (s *Service) PrepareRestore(archivePath string) (RestorePreparation, error) {
	result := RestorePreparation{}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return result, ErrBackupInvalidArchive
	}
	defer reader.Close()

	_ = os.RemoveAll("data/uploads_pending")
	_ = os.MkdirAll("data/uploads_pending", 0755)

	pendingFiles, _ := filepath.Glob("data/*.pending")
	for _, file := range pendingFiles {
		_ = os.Remove(file)
	}

	for _, file := range reader.File {
		if strings.Contains(file.Name, "..") {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			continue
		}

		switch {
		case file.Name == "nms.db":
			_ = os.MkdirAll("data", 0755)
			outFile, err := os.Create("data/nms.db.pending")
			if err == nil {
				_, _ = io.Copy(outFile, rc)
				_ = outFile.Close()
				result.HasDB = true
			}
		case file.Name == "nms.db-wal":
			outFile, err := os.Create("data/nms.db-wal.pending")
			if err == nil {
				_, _ = io.Copy(outFile, rc)
				_ = outFile.Close()
			}
		case file.Name == "nms.db-shm":
			outFile, err := os.Create("data/nms.db-shm.pending")
			if err == nil {
				_, _ = io.Copy(outFile, rc)
				_ = outFile.Close()
			}
		case file.Name == "config.yaml":
			_ = os.MkdirAll("config", 0755)
			outFile, err := os.Create("config/config.yaml.pending")
			if err == nil {
				_, _ = io.Copy(outFile, rc)
				_ = outFile.Close()
				result.ExtractedConfig = true
			}
		case filepath.HasPrefix(file.Name, "uploads/") || filepath.HasPrefix(file.Name, "data/uploads/"):
			relPath := file.Name
			if filepath.HasPrefix(relPath, "data/") {
				relPath = relPath[5:]
			}
			if len(relPath) <= len("uploads/") {
				_ = rc.Close()
				continue
			}
			innerName := relPath[len("uploads/"):]
			targetPath := filepath.Join("data/uploads_pending", innerName)
			if file.FileInfo().IsDir() {
				_ = os.MkdirAll(targetPath, file.Mode())
			} else {
				_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
				outFile, err := os.Create(targetPath)
				if err == nil {
					_, _ = io.Copy(outFile, rc)
					_ = outFile.Close()
					result.ExtractedUploads++
				}
			}
		}

		_ = rc.Close()
	}

	if !result.HasDB {
		return result, ErrBackupMissingDatabase
	}

	return result, nil
}

func (s *Service) CheckRestoreReadiness() RestoreReadiness {
	issues := []string{}

	if runtime.GOOS == "linux" {
		cmd := exec.Command("which", "sqlite3")
		if err := cmd.Run(); err != nil {
			issues = append(issues, "Linux 系統未安裝 sqlite3，還原前請先安裝 sqlite3")
		}
	}

	var walMode string
	_ = s.db.QueryRow("PRAGMA journal_mode").Scan(&walMode)

	if _, err := s.db.Exec("PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		issues = append(issues, fmt.Sprintf("WAL checkpoint 檢查失敗: %v", err))
	}

	return RestoreReadiness{
		Ready:   len(issues) == 0,
		Issues:  issues,
		WALMode: walMode,
		OS:      runtime.GOOS,
	}
}

func (s *Service) ExportEncryptedBackup(password string) (ArchiveResult, error) {
	if len(password) < 8 {
		return ArchiveResult{}, ErrBackupPasswordTooShort
	}

	encService, err := dbencryption.NewService(password)
	if err != nil {
		return ArchiveResult{}, err
	}

	timestamp := time.Now().Format("20060102_150405")
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("nms_backup_encrypted_%s.enc", timestamp))
	if err := encService.CreateEncryptedBackup(s.config.Database.Path, tempPath); err != nil {
		return ArchiveResult{}, err
	}

	filename := fmt.Sprintf("nms_backup_encrypted_%s.enc", timestamp)
	return ArchiveResult{
		Path:        tempPath,
		Filename:    filename,
		ContentType: "application/octet-stream",
	}, nil
}

func (s *Service) RestoreEncryptedBackup(encryptedPath, password string) error {
	if strings.TrimSpace(password) == "" {
		return ErrBackupPasswordRequired
	}

	decService, err := dbencryption.NewService(password)
	if err != nil {
		return err
	}

	if err := decService.ValidatePassword(encryptedPath); err != nil {
		return ErrEncryptedBackupWrongPassword
	}

	tempDecrypted := filepath.Join(os.TempDir(), fmt.Sprintf("nms_restore_%d.db", time.Now().UnixNano()))
	defer os.Remove(tempDecrypted)

	if err := decService.DecryptBackup(encryptedPath, tempDecrypted); err != nil {
		return err
	}

	decryptedData, err := os.ReadFile(tempDecrypted)
	if err != nil {
		return err
	}

	return os.WriteFile(s.config.Database.Path, decryptedData, 0644)
}

func (s *Service) RotateEncryptionPassword(newPassword string) error {
	if len(newPassword) < 8 {
		return ErrBackupPasswordTooShort
	}

	encService, err := dbencryption.NewService(newPassword)
	if err != nil {
		return err
	}

	backupPath := filepath.Join(os.TempDir(), fmt.Sprintf("nms_pre_encrypt_%s.enc", time.Now().Format("20060102_150405")))
	defer os.Remove(backupPath)

	if err := encService.CreateEncryptedBackup(s.config.Database.Path, backupPath); err != nil {
		return err
	}

	return nil
}

func (s *Service) SaveDeviceConfig(deviceID int) error {
	if s.snmp == nil {
		return ErrBackupCollectorUnavailable
	}
	return s.snmp.SaveDeviceConfig(deviceID)
}

func (s *Service) ListDeviceConfigBackups(deviceID int) ([]DeviceConfigBackup, error) {
	rows, err := s.db.Query(`
		SELECT id, device_id, COALESCE(note, ''), created_at, length(content)
		FROM device_config_backups
		WHERE device_id = ?
		ORDER BY id DESC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	backups := make([]DeviceConfigBackup, 0)
	for rows.Next() {
		var item DeviceConfigBackup
		if err := rows.Scan(&item.ID, &item.DeviceID, &item.Note, &item.CreatedAt, &item.Size); err != nil {
			return nil, err
		}
		backups = append(backups, item)
	}
	return backups, rows.Err()
}

func (s *Service) GetDeviceConfigBackup(deviceID, backupID int) (DeviceConfigBackupContent, error) {
	var item DeviceConfigBackupContent
	err := s.db.QueryRow(`
		SELECT id, device_id, created_at, COALESCE(note, ''), content
		FROM device_config_backups
		WHERE device_id = ? AND id = ?
	`, deviceID, backupID).Scan(&item.ID, &item.DeviceID, &item.CreatedAt, &item.Note, &item.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceConfigBackupContent{}, ErrDeviceConfigBackupNotFound
	}
	if err != nil {
		return DeviceConfigBackupContent{}, err
	}
	return item, nil
}

func Is(err, target error) bool {
	return errors.Is(err, target)
}
