const fs = require('fs');
const path = require('path');

const updates = {
    'zh-TW': {
        'cameras.status_online': '線上',
        'cameras.status_offline': '離線',
        'cameras.overlay_name': '攝影機名稱:',
        'cameras.overlay_ip': 'IP:',
        'cameras.overlay_stream': '串流模式:',
        'cameras.overlay_status': '狀態:',
        'cameras.overlay_location': '位置:',
        'cameras.drop_hint': '拖曳攝影機至此'
    },
    'zh-CN': {
        'cameras.status_online': '在线',
        'cameras.status_offline': '离线',
        'cameras.overlay_name': '摄像机名称:',
        'cameras.overlay_ip': 'IP:',
        'cameras.overlay_stream': '串流模式:',
        'cameras.overlay_status': '状态:',
        'cameras.overlay_location': '位置:',
        'cameras.drop_hint': '拖曳摄像机至此'
    },
    'en-US': {
        'cameras.status_online': 'Online',
        'cameras.status_offline': 'Offline',
        'cameras.overlay_name': 'Camera:',
        'cameras.overlay_ip': 'IP:',
        'cameras.overlay_stream': 'Stream:',
        'cameras.overlay_status': 'Status:',
        'cameras.overlay_location': 'Location:',
        'cameras.drop_hint': 'Drag camera here'
    }
};

['ko-KR', 'ja-JP'].forEach(lang => {
    updates[lang] = { ...updates['en-US'] };
});

const i18nDir = path.join(__dirname, 'frontend', 'i18n');

for (const lang in Object.keys(updates)) {
    const file = path.join(i18nDir, Object.keys(updates)[lang] + '.json');
    if (fs.existsSync(file)) {
        let content = JSON.parse(fs.readFileSync(file, 'utf8'));
        Object.assign(content, updates[Object.keys(updates)[lang]]);
        fs.writeFileSync(file, JSON.stringify(content, null, 2));
        console.log('Updated ' + file);
    }
}
