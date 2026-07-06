// Made by YTSworks
// YTS工作室製作
const fs = require('fs');
const path = require('path');

const i18nDir = path.join(__dirname, 'frontend', 'i18n');

// Complete translations for all new keys
const translations = {
    'en-US.json': {
        common: {
            just_now: "Just now",
            minutes_ago: "minutes ago",
            hours_ago: "hours ago",
            days_ago: "days ago",
            success: "Success",
            failed: "Failed",
            error: "Error",
            warning: "Warning",
            confirm_delete: "Are you sure you want to delete?",
            operation_success: "Operation successful",
            operation_failed: "Operation failed",
            loading: "Loading...",
            please_wait: "Please wait"
        },
        devices: {
            toast: {
                load_failed: "Failed to load device list",
                input_ip_range: "Please enter start and end IP",
                input_subnet: "Please enter network segment",
                scan_failed: "Scan failed",
                load_detail_failed: "Unable to load device details",
                command_sent: "Command sent successfully",
                operation_failed: "Operation failed",
                reboot_sent: "Reboot command sent",
                reboot_failed: "Reboot failed",
                backup_sent: "Save configuration command sent",
                backup_failed: "Backup failed",
                poe_action_sent: "PoE {action} command sent",
                update_success: "Device updated successfully",
                update_failed: "Update failed",
                select_image: "Please select an image",
                image_upload_success: "Image uploaded successfully",
                upload_failed: "Upload failed",
                invalid_device_id: "Error: Invalid device ID",
                delete_success: "Device deleted successfully",
                delete_failed: "Delete failed",
                rescan_sent: "Rescan command sent",
                rescan_failed: "Rescan failed",
                image_delete_success: "Image deleted"
            },
            confirm: {
                delete_device: "Are you sure you want to delete this device?",
                rescan_device: "Are you sure you want to rescan device information now?",
                delete_image: "Are you sure you want to delete this device's custom image?"
            }
        },
        topology: {
            toast: {
                load_failed: "Failed to load topology data",
                device_placed: "Device placement successful",
                update_position_failed: "Unable to update device position",
                select_source: "Source device selected: {name}, please click target device",
                cannot_self_connect: "Cannot connect to the same device",
                manual_mode_hint: "Manual link mode: Please click source device",
                link_created: "Link created successfully",
                create_link_failed: "Failed to create link",
                link_deleted: "Link deleted",
                delete_failed: "Delete failed",
                no_devices_to_save: "No devices to save",
                positions_saved: "Positions saved",
                save_positions_failed: "Failed to save positions",
                discovery_started: "Topology discovery started",
                discovery_failed: "Failed to start topology discovery",
                link_info: "Link: {source} ↔ {target}",
                device_removed: "Device removed from topology",
                remove_device_failed: "Failed to remove device",
                delete_link_failed: "Failed to delete link"
            },
            confirm: {
                remove_device: "Are you sure you want to remove \"{name}\" from the topology?\\n\\nThe device will be moved back to the pending deployment list but will not be deleted from the system.",
                delete_link: "Are you sure you want to delete the link between \"{source}\" and \"{target}\"?"
            },
            modal: {
                manual_link_title: "Create Manual Link",
                link_detail_title: "Link Details & Management"
            },
            drag_hint: "Please drag devices from the left panel to here",
            expand: "Expand",
            collapse: "Collapse"
        },
        logs: {
            toast: {
                load_failed: "Failed to load logs"
            }
        },
        admin: {
            reports: {
                traffic_exporting: "Exporting traffic report...",
                health_exporting: "Exporting health report...",
                availability_exporting: "Exporting availability report...",
                inventory_exporting: "Exporting inventory list..."
            },
            toast: {
                load_host_failed: "Failed to load host status",
                auth_failed: "Authentication failed: Invalid username or password",
                load_alert_failed: "Failed to load alert settings",
                save_failed: "Save failed",
                test_failed: "Test failed",
                logo_deleted: "Logo deleted",
                refreshing: "Refreshing..."
            },
            confirm: {
                delete_logo: "Are you sure you want to delete the current logo?",
                reset_backup: "Warning: The system will be restored to the backup state, all current settings will be overwritten!\\n\\nAre you sure you want to continue?",
                clear_license: "Warning: This will clear all license data, including machine ID and license key!\\n\\nAre you sure you want to proceed?"
            }
        },
        utils: {
            cli: {
                admin_account: "Admin Account",
                device_password: "Device Password",
                password_placeholder: "Please enter password",
                cancel: "Cancel",
                execute: "Execute Command",
                input_required: "Please enter complete account and password"
            }
        }
    },
    'zh-CN.json': {
        common: {
            just_now: "刚刚",
            minutes_ago: "分钟前",
            hours_ago: "小时前",
            days_ago: "天前",
            success: "成功",
            failed: "失败",
            error: "错误",
            warning: "警告",
            confirm_delete: "确定要删除吗？",
            operation_success: "操作成功",
            operation_failed: "操作失败",
            loading: "加载中...",
            please_wait: "请稍候"
        },
        devices: {
            toast: {
                load_failed: "加载设备清单失败",
                input_ip_range: "请输入起始和结束 IP",
                input_subnet: "请输入子网段",
                scan_failed: "扫描失败",
                load_detail_failed: "无法加载设备详情",
                command_sent: "指令已发送成功",
                operation_failed: "操作失败",
                reboot_sent: "重启指令已发送",
                reboot_failed: "重启失败",
                backup_sent: "设定储存指令已发送",
                backup_failed: "备份失败",
                poe_action_sent: "PoE {action} 指令已发送",
                update_success: "设备更新成功",
                update_failed: "更新失败",
                select_image: "请选择图片",
                image_upload_success: "图片上传成功",
                upload_failed: "上传失败",
                invalid_device_id: "错误：无法识别设备 ID",
                delete_success: "设备删除成功",
                delete_failed: "删除失败",
                rescan_sent: "重新扫描指令已发送",
                rescan_failed: "重新扫描失败",
                image_delete_success: "图片已删除"
            },
            confirm: {
                delete_device: "确定要删除此设备吗？",
                rescan_device: "确定要立即重新扫描设备资讯吗？",
                delete_image: "确定要删除此设备的自订图片吗？"
            }
        },
        topology: {
            toast: {
                load_failed: "加载拓扑资料失败",
                device_placed: "设备已放置",
                update_position_failed: "无法更新设备位置",
                select_source: "已选择来源设备: {name}，请点击目标设备",
                cannot_self_connect: "不能连接到同一设备",
                manual_mode_hint: "手动连线模式：请点击来源设备",
                link_created: "连线建立成功",
                create_link_failed: "建立连线失败",
                link_deleted: "连线已删除",
                delete_failed: "删除失败",
                no_devices_to_save: "没有设备可储存",
                positions_saved: "位置已储存",
                save_positions_failed: "储存位置失败",
                discovery_started: "拓扑发现已启动",
                discovery_failed: "启动拓扑发现失败",
                link_info: "连线: {source} ↔ {target}",
                device_removed: "设备已从拓扑移除",
                remove_device_failed: "移除设备失败",
                delete_link_failed: "删除连线失败"
            },
            confirm: {
                remove_device: "确定要将"{ name }"从拓扑图中移除吗？\\n\\n设备将移回待布署列表，但不会从系统中删除。",
                    delete_link: "确定要删除"{ source } "与"{ target } "之间的连线吗？"
            },
modal: {
    manual_link_title: "建立手动连线",
        link_detail_title: "连线详情与管理"
},
drag_hint: "请从左侧拖曳设备到此处",
    expand: "展开",
        collapse: "收合"
        },
logs: {
    toast: {
        load_failed: "加载日志失败"
    }
},
admin: {
    reports: {
        traffic_exporting: "流量报表汇出中...",
            health_exporting: "健康度报表汇出中...",
                availability_exporting: "可用性报表汇出中...",
                    inventory_exporting: "资产清单汇出中..."
    },
    toast: {
        load_host_failed: "加载主机状态失败",
            auth_failed: "验证失败: 账号或是密码错误",
                load_alert_failed: "加载告警设定失败",
                    save_failed: "储存失败",
                        test_failed: "测试失败",
                            logo_deleted: "Logo 已删除",
                                refreshing: "重新整理中..."
    },
    confirm: {
        delete_logo: "确定要删除目前的 Logo 吗？",
            reset_backup: "警告：系统将会还原至备份状态，所有目前设定将被覆盖！\\n\\n确定要继续吗？",
                clear_license: "警告：这将会清除所有授权资料，包含机器 ID 与授权金钥！\\n\\n确定要执行清除吗？"
    }
},
utils: {
    cli: {
        admin_account: "管理员账号",
            device_password: "设备密码",
                password_placeholder: "请输入密码",
                    cancel: "取消",
                        execute: "执行指令",
                            input_required: "请输入完整的账号密码"
    }
}
    },
'ja-JP.json': {
    common: {
        just_now: "たった今",
            minutes_ago: "分前",
                hours_ago: "時間前",
                    days_ago: "日前",
                        success: "成功",
                            failed: "失敗",
                                error: "エラー",
                                    warning: "警告",
                                        confirm_delete: "削除してもよろしいですか？",
                                            operation_success: "操作が成功しました",
                                                operation_failed: "操作が失敗しました",
                                                    loading: "読み込み中...",
                                                        please_wait: "お待ちください"
    },
    devices: {
        toast: {
            load_failed: "デバイスリストの読み込みに失敗しました",
                input_ip_range: "開始IPと終了IPを入力してください",
                    input_subnet: "サブネットを入力してください",
                        scan_failed: "スキャンに失敗しました",
                            load_detail_failed: "デバイス詳細を読み込めませんでした",
                                command_sent: "コマンドが送信されました",
                                    operation_failed: "操作に失敗しました",
                                        reboot_sent: "再起動コマンドが送信されました",
                                            reboot_failed: "再起動に失敗しました",
                                                backup_sent: "設定保存コマンドが送信されました",
                                                    backup_failed: "バックアップに失敗しました",
                                                        poe_action_sent: "PoE {action} コマンドが送信されました",
                                                            update_success: "デバイスが更新されました",
                                                                update_failed: "更新に失敗しました",
                                                                    select_image: "画像を選択してください",
                                                                        image_upload_success: "画像がアップロードされました",
                                                                            upload_failed: "アップロードに失敗しました",
                                                                                invalid_device_id: "エラー: デバイスIDを認識できません",
                                                                                    delete_success: "デバイスが削除されました",
                                                                                        delete_failed: "削除に失敗しました",
                                                                                            rescan_sent: "再スキャンコマンドが送信されました",
                                                                                                rescan_failed: "再スキャンに失敗しました",
                                                                                                    image_delete_success: "画像が削除されました"
        },
        confirm: {
            delete_device: "このデバイスを削除してもよろしいですか？",
                rescan_device: "デバイス情報を今すぐ再スキャンしてもよろしいですか？",
                    delete_image: "このデバイスのカスタム画像を削除してもよろしいですか？"
        }
    },
    topology: {
        toast: {
            load_failed: "トポロジーデータの読み込みに失敗しました",
                device_placed: "デバイスが配置されました",
                    update_position_failed: "デバイスの位置を更新できませんでした",
                        select_source: "ソースデバイスを選択しました: {name}、ターゲットデバイスをクリックしてください",
                            cannot_self_connect: "同じデバイスには接続できません",
                                manual_mode_hint: "手動リンクモード: ソースデバイスをクリックしてください",
                                    link_created: "リンクが作成されました",
                                        create_link_failed: "リンクの作成に失敗しました",
                                            link_deleted: "リンクが削除されました",
                                                delete_failed: "削除に失敗しました",
                                                    no_devices_to_save: "保存するデバイスがありません",
                                                        positions_saved: "位置が保存されました",
                                                            save_positions_failed: "位置の保存に失敗しました",
                                                                discovery_started: "トポロジー検出が開始されました",
                                                                    discovery_failed: "トポロジー検出の開始に失敗しました",
                                                                        link_info: "リンク: {source} ↔ {target}",
                                                                            device_removed: "デバイスがトポロジーから削除されました",
                                                                                remove_device_failed: "デバイスの削除に失敗しました",
                                                                                    delete_link_failed: "リンクの削除に失敗しました"
        },
        confirm: {
            remove_device: "トポロジーから「{name}」を削除してもよろしいですか？\\n\\nデバイスは保留中のデプロイリストに移動されますが、システムからは削除されません。",
                delete_link: "「{source}」と「{target}」の間のリンクを削除してもよろしいですか？"
        },
        modal: {
            manual_link_title: "手動リンクの作成",
                link_detail_title: "リンク詳細と管理"
        },
        drag_hint: "左パネルからデバイスをここにドラッグしてください",
            expand: "展開",
                collapse: "折りたたむ"
    },
    logs: {
        toast: {
            load_failed: "ログの読み込みに失敗しました"
        }
    },
    admin: {
        reports: {
            traffic_exporting: "トラフィックレポートをエクスポート中...",
                health_exporting: "ヘルスレポートをエクスポート中...",
                    availability_exporting: "可用性レポートをエクスポート中...",
                        inventory_exporting: "インベントリリストをエクスポート中..."
        },
        toast: {
            load_host_failed: "ホストステータスの読み込みに失敗しました",
                auth_failed: "認証に失敗しました: ユーザー名またはパスワードが無効です",
                    load_alert_failed: "アラート設定の読み込みに失敗しました",
                        save_failed: "保存に失敗しました",
                            test_failed: "テストに失敗しました",
                                logo_deleted: "ロゴが削除されました",
                                    refreshing: "更新中..."
        },
        confirm: {
            delete_logo: "現在のロゴを削除してもよろしいですか？",
                reset_backup: "警告: システムはバックアップ状態に復元され、すべての現在の設定が上書きされます！\\n\\n続行してもよろしいですか？",
                    clear_license: "警告: これにより、マシンIDとライセンスキーを含むすべてのライセンスデータがクリアされます！\\n\\n実行してもよろしいですか？"
        }
    },
    utils: {
        cli: {
            admin_account: "管理者アカウント",
                device_password: "デバイスパスワード",
                    password_placeholder: "パスワードを入力してください",
                        cancel: "キャンセル",
                            execute: "コマンド実行",
                                input_required: "完全なアカウントとパスワードを入力してください"
        }
    }
},
'ko-KR.json': {
    common: {
        just_now: "방금",
            minutes_ago: "분 전",
                hours_ago: "시간 전",
                    days_ago: "일 전",
                        success: "성공",
                            failed: "실패",
                                error: "오류",
                                    warning: "경고",
                                        confirm_delete: "삭제하시겠습니까?",
                                            operation_success: "작업 성공",
                                                operation_failed: "작업 실패",
                                                    loading: "로딩 중...",
                                                        please_wait: "잠시 기다려주세요"
    },
    devices: {
        toast: {
            load_failed: "장치 목록 로드 실패",
                input_ip_range: "시작 및 종료 IP를 입력하세요",
                    input_subnet: "네트워크 세그먼트를 입력하세요",
                        scan_failed: "스캔 실패",
                            load_detail_failed: "장치 세부 정보를 로드할 수 없습니다",
                                command_sent: "명령이 전송되었습니다",
                                    operation_failed: "작업 실패",
                                        reboot_sent: "재부팅 명령이 전송되었습니다",
                                            reboot_failed: "재부팅 실패",
                                                backup_sent: "구성 저장 명령이 전송되었습니다",
                                                    backup_failed: "백업 실패",
                                                        poe_action_sent: "PoE {action} 명령이 전송되었습니다",
                                                            update_success: "장치가 업데이트되었습니다",
                                                                update_failed: "업데이트 실패",
                                                                    select_image: "이미지를 선택하세요",
                                                                        image_upload_success: "이미지가 업로드되었습니다",
                                                                            upload_failed: "업로드 실패",
                                                                                invalid_device_id: "오류: 잘못된 장치 ID",
                                                                                    delete_success: "장치가 삭제되었습니다",
                                                                                        delete_failed: "삭제 실패",
                                                                                            rescan_sent: "재스캔 명령이 전송되었습니다",
                                                                                                rescan_failed: "재스캔 실패",
                                                                                                    image_delete_success: "이미지가 삭제되었습니다"
        },
        confirm: {
            delete_device: "이 장치를 삭제하시겠습니까?",
                rescan_device: "지금 장치 정보를 다시 스캔하시겠습니까?",
                    delete_image: "이 장치의 사용자 정의 이미지를 삭제하시겠습니까?"
        }
    },
    topology: {
        toast: {
            load_failed: "토폴로지 데이터 로드 실패",
                device_placed: "장치가 배치되었습니다",
                    update_position_failed: "장치 위치를 업데이트할 수 없습니다",
                        select_source: "소스 장치 선택됨: {name}, 대상 장치를 클릭하세요",
                            cannot_self_connect: "동일한 장치에 연결할 수 없습니다",
                                manual_mode_hint: "수동 링크 모드: 소스 장치를 클릭하세요",
                                    link_created: "링크가 생성되었습니다",
                                        create_link_failed: "링크 생성 실패",
                                            link_deleted: "링크가 삭제되었습니다",
                                                delete_failed: "삭제 실패",
                                                    no_devices_to_save: "저장할 장치가 없습니다",
                                                        positions_saved: "위치가 저장되었습니다",
                                                            save_positions_failed: "위치 저장 실패",
                                                                discovery_started: "토폴로지 검색이 시작되었습니다",
                                                                    discovery_failed: "토폴로지 검색 시작 실패",
                                                                        link_info: "링크: {source} ↔ {target}",
                                                                            device_removed: "토폴로지에서 장치가 제거되었습니다",
                                                                                remove_device_failed: "장치 제거 실패",
                                                                                    delete_link_failed: "링크 삭제 실패"
        },
        confirm: {
            remove_device: "토폴로지에서 \"{name}\"을(를) 제거하시겠습니까?\\n\\n장치는 대기 중인 배포 목록으로 다시 이동되지만 시스템에서 삭제되지는 않습니다.",
                delete_link: "\"{source}\"와 \"{target}\" 사이의 링크를 삭제하시겠습니까?"
        },
        modal: {
            manual_link_title: "수동 링크 생성",
                link_detail_title: "링크 세부 정보 및 관리"
        },
        drag_hint: "왼쪽 패널에서 장치를 여기로 드래그하세요",
            expand: "확장",
                collapse: "축소"
    },
    logs: {
        toast: {
            load_failed: "로그 로드 실패"
        }
    },
    admin: {
        reports: {
            traffic_exporting: "트래픽 보고서 내보내는 중...",
                health_exporting: "상태 보고서 내보내는 중...",
                    availability_exporting: "가용성 보고서 내보내는 중...",
                        inventory_exporting: "인벤토리 목록 내보내는 중..."
        },
        toast: {
            load_host_failed: "호스트 상태 로드 실패",
                auth_failed: "인증 실패: 잘못된 사용자 이름 또는 비밀번호",
                    load_alert_failed: "알림 설정 로드 실패",
                        save_failed: "저장 실패",
                            test_failed: "테스트 실패",
                                logo_deleted: "로고가 삭제되었습니다",
                                    refreshing: "새로 고침 중..."
        },
        confirm: {
            delete_logo: "현재 로고를 삭제하시겠습니까?",
                reset_backup: "경고: 시스템이 백업 상태로 복원되고 현재 모든 설정이 덮어쓰여집니다!\\n\\n계속하시겠습니까?",
                    clear_license: "경고: 머신 ID 및 라이센스 키를 포함한 모든 라이센스 데이터가 삭제됩니다!\\n\\n실행하시겠습니까?"
        }
    },
    utils: {
        cli: {
            admin_account: "관리자 계정",
                device_password: "장치 비밀번호",
                    password_placeholder: "비밀번호를 입력하세요",
                        cancel: "취소",
                            execute: "명령 실행",
                                input_required: "완전한 계정과 비밀번호를 입力하세요"
        }
    }
}
};

// Deep merge function
function deepMerge(target, source) {
    for (const key in source) {
        if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key])) {
            if (!target[key]) target[key] = {};
            deepMerge(target[key], source[key]);
        } else {
            target[key] = source[key];
        }
    }
}

// Apply translations to all language files
Object.keys(translations).forEach(filename => {
    const filePath = path.join(i18nDir, filename);
    let data = {};

    if (fs.existsSync(filePath)) {
        data = JSON.parse(fs.readFileSync(filePath, 'utf8'));
    }

    // Deep merge translations
    deepMerge(data, translations[filename]);

    fs.writeFileSync(filePath, JSON.stringify(data, null, 4), 'utf8');
    console.log(`✓ Updated ${filename}`);
});

console.log('\n✓ All translations added successfully!');
