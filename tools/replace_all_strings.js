// Made by YTSworks
// YTS工作室製作
// Comprehensive string replacement mapping for all JS files
const fs = require('fs');
const path = require('path');

// Mapping: Chinese string -> translation key
const replacements = {
    // utils.js - Time formats
    "'剛剛'": "t('common.just_now')",
    "' 分鐘前'": "' ' + t('common.minutes_ago')",
    "' 小時前'": "' ' + t('common.hours_ago')",
    "' 天前'": "' ' + t('common.days_ago')",
    "'請輸入完整的帳號密碼'": "t('utils.cli.input_required')",

    // CLI dialog labels in utils.js
    "管理員帳號": "${t('utils.cli.admin_account')}",
    "設備密碼": "${t('utils.cli.device_password')}",
    "請輸入密碼": "${t('utils.cli.password_placeholder')}",
    ">取消</button>": ">${t('utils.cli.cancel')}</button>",
    ">執行指令</button>": ">${t('utils.cli.execute')}</button>",

    // topology.js - Toast messages
    "'載入拓樸資料失敗'": "t('topology.toast.load_failed')",
    "'設備已放置'": "t('topology.toast.device_placed')",
    "'無法更新設備位置'": "t('topology.toast.update_position_failed')",
    "`已選擇來源設備: ${d.name}，請點擊目標設備`": "t('topology.toast.select_source').replace('{name}', d.name)",
    "'不能連接到同一設備'": "t('topology.toast.cannot_self_connect')",
    "'手動連線模式：請點擊來源設備'": "t('topology.toast.manual_mode_hint')",
    "'連線建立成功'": "t('topology.toast.link_created')",
    "'建立連線失敗'": "t('topology.toast.create_link_failed')",
    "'連線已刪除'": "t('topology.toast.link_deleted')",
    "'刪除失敗'": "t('topology.toast.delete_failed')",
    "'沒有設備可儲存'": "t('topology.toast.no_devices_to_save')",
    "'位置已儲存'": "t('topology.toast.positions_saved')",
    "'儲存位置失敗'": "t('topology.toast.save_positions_failed')",
    "'拓樸發現已啟動'": "t('topology.toast.discovery_started')",
    "'啟動拓樸發現失敗'": "t('topology.toast.discovery_failed')",
    "`連線: ${sourceName} ↔ ${targetName}`": "t('topology.toast.link_info').replace('{source}', sourceName).replace('{target}', targetName)",
    "'設備已從拓樸移除'": "t('topology.toast.device_removed')",
    "'移除設備失敗'": "t('topology.toast.remove_device_failed')",
    "'刪除連線失敗'": "t('topology.toast.delete_link_failed')",

    // topology.js - Modal titles
    "'建立手動連線'": "'topology.modal.manual_link_title'",
    "'連線詳情與管理'": "'topology.modal.link_detail_title'",
    "'請從左側拖曳設備到此處'": "t('topology.drag_hint')",
    "'展開'": "t('topology.expand')",
    "'收合'": "t('topology.collapse')",

    // topology.js - Confirm dialogs
    "`確定要將「${deviceName}」從拓樸圖中移除嗎？\\n\\n設備將移回待佈署列表，但不會從系統中刪除。`": "t('topology.confirm.remove_device').replace('{name}', deviceName)",
    "`確定要刪除「${sourceName}」與「${targetName}」之間的連線嗎？`": "t('topology.confirm.delete_link').replace('{source}', sourceName).replace('{target}', targetName)",

    // devices.js - Toast messages
    "'載入設備清單失敗'": "t('devices.toast.load_failed')",
    "'請輸入起始和結束 IP'": "t('devices.toast.input_ip_range')",
    "'請輸入子網段'": "t('devices.toast.input_subnet')",
    "'掃描失敗'": "t('devices.toast.scan_failed')",
    "'無法載入設備詳情'": "t('devices.toast.load_detail_failed')",
    "'指令已發送成功'": "t('devices.toast.command_sent')",
    "'操作失敗'": "t('devices.toast.operation_failed')",
    "'重啟指令已發送'": "t('devices.toast.reboot_sent')",
    "'重啟失敗'": "t('devices.toast.reboot_failed')",
    "'設定儲存指令已發送'": "t('devices.toast.backup_sent')",
    "'備份失敗'": "t('devices.toast.backup_failed')",
    "`PoE ${actionText} 指令已發送`": "t('devices.toast.poe_action_sent').replace('{action}', actionText)",
    "'設備更新成功'": "t('devices.toast.update_success')",
    "'更新失敗'": "t('devices.toast.update_failed')",
    "'請選擇圖片'": "t('devices.toast.select_image')",
    "'圖片上傳成功'": "t('devices.toast.image_upload_success')",
    "'上傳失敗'": "t('devices.toast.upload_failed')",
    "'錯誤：無法識別設備 ID'": "t('devices.toast.invalid_device_id')",
    "'設備刪除成功'": "t('devices.toast.delete_success')",
    "'刪除失敗'": "t('devices.toast.delete_failed')",
    "'重新掃描指令已發送'": "t('devices.toast.rescan_sent')",
    "'重新掃描失敗'": "t('devices.toast.rescan_failed')",
    "'圖片已刪除'": "t('devices.toast.image_delete_success')",

    // devices.js - Confirm dialogs
    "'確定要刪除此設備嗎？'": "t('devices.confirm.delete_device')",
    "'確定要立即重新掃描設備資訊嗎？'": "t('devices.confirm.rescan_device')",
    "'確定要刪除此設備的自訂圖片嗎？'": "t('devices.confirm.delete_image')",

    // logs.js
    "'載入日誌失敗'": "t('logs.toast.load_failed')",

    // reports.js
    "'流量報表匯出中...'": "t('admin.reports.traffic_exporting')",
    "'健康度報表匯出中...'": "t('admin.reports.health_exporting')",
    "'可用性報表匯出中...'": "t('admin.reports.availability_exporting')",
    "'資產清單匯出中...'": "t('admin.reports.inventory_exporting')",

    // admin.js - Toast messages
    "'載入主機狀態失敗'": "t('admin.toast.load_host_failed')",
    "'驗證失敗: 帳號或是密碼錯誤'": "t('admin.toast.auth_failed')",
    "'載入告警設定失敗'": "t('admin.toast.load_alert_failed')",
    "'儲存失敗'": "t('admin.toast.save_failed')",
    "'測試失敗'": "t('admin.toast.test_failed')",
    "'Logo 已刪除'": "t('admin.toast.logo_deleted')",
    "'重新整理中...'": "t('admin.toast.refreshing')",

    // admin.js - Confirm dialogs
    "'確定要刪除目前的 Logo 嗎？'": "t('admin.confirm.delete_logo')",
    "'警告：系統將會還原至備份狀態，所有目前設定將被覆蓋！\\n\\n確定要繼續嗎？'": "t('admin.confirm.reset_backup')",
    "'警告：這將會清除所有授權資料，包含機器 ID 與授權金鑰！\\n\\n確定要執行清除嗎？'": "t('admin.confirm.clear_license')"
};

// Apply replacements to a file
function processFile(filePath, mapping) {
    if (!fs.existsSync(filePath)) {
        console.log(`⚠ File not found: ${filePath}`);
        return 0;
    }

    let content = fs.readFileSync(filePath, 'utf8');
    let count = 0;

    Object.keys(mapping).forEach(search => {
        const replace = mapping[search];
        const before = content;
        content = content.split(search

        ).join(replace);
        if (content !== before) count++;
    });

    if (count > 0) {
        fs.writeFileSync(filePath, content, 'utf8');
        console.log(`✓ ${path.basename(filePath)}: ${count} replacements`);
    } else {
        console.log(`  ${path.basename(filePath)}: No changes`);
    }

    return count;
}

// Process all JS files
const jsDir = path.join(__dirname, 'frontend', 'js');
const files = [
    'utils.js',
    'topology.js',
    'devices.js',
    'logs.js',
    'reports.js',
    'admin.js'
];

console.log('Starting JS file replacements...\n');
let totalReplacements = 0;

files.forEach(file => {
    const filePath = path.join(jsDir, file);
    totalReplacements += processFile(filePath, replacements);
});

console.log(`\n✓ Total ${totalReplacements} string replacements completed!`);
console.log('All JS files updated to use t() function.');
