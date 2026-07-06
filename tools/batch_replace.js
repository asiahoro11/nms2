// Made by YTSworks
// YTS工作室製作
const fs = require('fs');

// topology.js
const f1 = 'frontend/js/topology.js';
let c1 = fs.readFileSync(f1, 'utf8');

c1 = c1.replace(/'載入拓樸資料失敗'/g, "t('topology.toast.load_failed')");
c1 = c1.replace(/'設備已放置'/g, "t('topology.toast.device_placed')");
c1 = c1.replace(/'無法更新設備位置'/g, "t('topology.toast.update_position_failed')");
c1 = c1.replace(/'不能連接到同一設備'/g, "t('topology.toast.cannot_self_connect')");
c1 = c1.replace(/'手動連線模式：請點擊來源設備'/g, "t('topology.toast.manual_mode_hint')");
c1 = c1.replace(/'連線建立成功'/g, "t('topology.toast.link_created')");
c1 = c1.replace(/'建立連線失敗'/g, "t('topology.toast.create_link_failed')");
c1 = c1.replace(/'連線已刪除'/g, "t('topology.toast.link_deleted')");
c1 = c1.replace(/'刪除失敗'/g, "t('topology.toast.delete_failed')");
c1 = c1.replace(/'沒有設備可儲存'/g, "t('topology.toast.no_devices_to_save')");
c1 = c1.replace(/'位置已儲存'/g, "t('topology.toast.positions_saved')");
c1 = c1.replace(/'儲存位置失敗'/g, "t('topology.toast.save_positions_failed')");
c1 = c1.replace(/'拓樸發現已啟動'/g, "t('topology.toast.discovery_started')");
c1 = c1.replace(/'啟動拓樸發現失敗'/g, "t('topology.toast.discovery_failed')");
c1 = c1.replace(/'設備已從拓樸移除'/g, "t('topology.toast.device_removed')");
c1 = c1.replace(/'移除設備失敗'/g, "t('topology.toast.remove_device_failed')");
c1 = c1.replace(/'刪除連線失敗'/g, "t('topology.toast.delete_link_failed')");
c1 = c1.replace(/'建立手動連線'/g, "'topology.modal.manual_link_title'");
c1 = c1.replace(/'連線詳情與管理'/g, "'topology.modal.link_detail_title'");
c1 = c1.replace(/'請從左側拖曳設備到此處'/g, "t('topology.drag_hint')");
c1 = c1.replace(/'展開'/g, "t('topology.expand')");
c1 = c1.replace(/'收合'/g, "t('topology.collapse')");

// Complex templates
c1 = c1.replace(/`已選擇來源設備: \${d\.name}，請點擊目標設備`/g, "t('topology.toast.select_source').replace('{name}', d.name)");
c1 = c1.replace(/`連線: \${sourceName} ↔ \${targetName}`/g, "t('topology.toast.link_info').replace('{source}', sourceName).replace('{target}', targetName)");
c1 = c1.replace(/`確定要將「\${deviceName}」從拓樸圖中移除嗎？\\n\\n設備將移回待佈署列表，但不會從系統中刪除。`/g, "t('topology.confirm.remove_device').replace('{name}', deviceName)");
c1 = c1.replace(/`確定要刪除「\${sourceName}」與「\${targetName}」之間的連線嗎？`/g, "t('topology.confirm.delete_link').replace('{source}', sourceName).replace('{target}', targetName)");

fs.writeFileSync(f1, c1, 'utf8');
console.log('✓ topology.js');

// devices.js
const f2 = 'frontend/js/devices.js';
let c2 = fs.readFileSync(f2, 'utf8');

c2 = c2.replace(/'載入設備清單失敗'/g, "t('devices.toast.load_failed')");
c2 = c2.replace(/'請輸入起始和結束 IP'/g, "t('devices.toast.input_ip_range')");
c2 = c2.replace(/'請輸入子網段'/g, "t('devices.toast.input_subnet')");
c2 = c2.replace(/'掃描失敗'/g, "t('devices.toast.scan_failed')");
c2 = c2.replace(/'無法載入設備詳情'/g, "t('devices.toast.load_detail_failed')");
c2 = c2.replace(/'指令已發送成功'/g, "t('devices.toast.command_sent')");
c2 = c2.replace(/'操作失敗'/g, "t('devices.toast.operation_failed')");
c2 = c2.replace(/'重啟指令已發送'/g, "t('devices.toast.reboot_sent')");
c2 = c2.replace(/'重啟失敗'/g, "t('devices.toast.reboot_failed')");
c2 = c2.replace(/'設定儲存指令已發送'/g, "t('devices.toast.backup_sent')");
c2 = c2.replace(/'備份失敗'/g, "t('devices.toast.backup_failed')");
c2 = c2.replace(/'設備更新成功'/g, "t('devices.toast.update_success')");
c2 = c2.replace(/'更新失敗'/g, "t('devices.toast.update_failed')");
c2 = c2.replace(/'請選擇圖片'/g, "t('devices.toast.select_image')");
c2 = c2.replace(/'圖片上傳成功'/g, "t('devices.toast.image_upload_success')");
c2 = c2.replace(/'上傳失敗'/g, "t('devices.toast.upload_failed')");
c2 = c2.replace(/'錯誤：無法識別設備 ID'/g, "t('devices.toast.invalid_device_id')");
c2 = c2.replace(/'設備刪除成功'/g, "t('devices.toast.delete_success')");
c2 = c2.replace(/'刪除失敗'/g, "t('devices.toast.delete_failed')");
c2 = c2.replace(/'重新掃描指令已發送'/g, "t('devices.toast.rescan_sent')");
c2 = c2.replace(/'重新掃描失敗'/g, "t('devices.toast.rescan_failed')");
c2 = c2.replace(/'圖片已刪除'/g, "t('devices.toast.image_delete_success')");
c2 = c2.replace(/'確定要刪除此設備嗎？'/g, "t('devices.confirm.delete_device')");
c2 = c2.replace(/'確定要立即重新掃描設備資訊嗎？'/g, "t('devices.confirm.rescan_device')");
c2 = c2.replace(/'確定要刪除此設備的自訂圖片嗎？'/g, "t('devices.confirm.delete_image')");
c2 = c2.replace(/`PoE \${actionText} 指令已發送`/g, "t('devices.toast.poe_action_sent').replace('{action}', actionText)");

fs.writeFileSync(f2, c2, 'utf8');
console.log('✓ devices.js');

// logs.js
const f3 = 'frontend/js/logs.js';
let c3 = fs.readFileSync(f3, 'utf8');
c3 = c3.replace(/'載入日誌失敗'/g, "t('logs.toast.load_failed')");
fs.writeFileSync(f3, c3, 'utf8');
console.log('✓ logs.js');

// reports.js
const f4 = 'frontend/js/reports.js';
let c4 = fs.readFileSync(f4, 'utf8');
c4 = c4.replace(/'流量報表匯出中\.\.\.'/g, "t('admin.reports.traffic_exporting')");
c4 = c4.replace(/'健康度報表匯出中\.\.\.'/g, "t('admin.reports.health_exporting')");
c4 = c4.replace(/'可用性報表匯出中\.\.\.'/g, "t('admin.reports.availability_exporting')");
c4 = c4.replace(/'資產清單匯出中\.\.\.'/g, "t('admin.reports.inventory_exporting')");
fs.writeFileSync(f4, c4, 'utf8');
console.log('✓ reports.js');

// admin.js
const f5 = 'frontend/js/admin.js';
let c5 = fs.readFileSync(f5, 'utf8');
c5 = c5.replace(/'載入主機狀態失敗'/g, "t('admin.toast.load_host_failed')");
c5 = c5.replace(/'驗證失敗: 帳號或是密碼錯誤'/g, "t('admin.toast.auth_failed')");
c5 = c5.replace(/'載入告警設定失敗'/g, "t('admin.toast.load_alert_failed')");
c5 = c5.replace(/'儲存失敗'/g, "t('admin.toast.save_failed')");
c5 = c5.replace(/'測試失敗'/g, "t('admin.toast.test_failed')");
c5 = c5.replace(/'Logo 已刪除'/g, "t('admin.toast.logo_deleted')");
c5 = c5.replace(/'重新整理中\.\.\.'/g, "t('admin.toast.refreshing')");
c5 = c5.replace(/'確定要刪除目前的 Logo 嗎？'/g, "t('admin.confirm.delete_logo')");
c5 = c5.replace(/'警告：系統將會還原至備份狀態，所有目前設定將被覆蓋！\\n\\n確定要繼續嗎？'/g, "t('admin.confirm.reset_backup')");
c5 = c5.replace(/'警告：這將會清除所有授權資料，包含機器 ID 與授權金鑰！\\n\\n確定要執行清除嗎？'/g, "t('admin.confirm.clear_license')");
fs.writeFileSync(f5, c5, 'utf8');
console.log('✓ admin.js');

console.log('\n✅ All JS files updated successfully!');
