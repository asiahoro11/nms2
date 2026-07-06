// Made by YTSworks
// YTS工作室製作
const fs = require('fs');
const path = require('path');

const i18nDir = 'frontend/i18n';
const languages = ['zh-TW.json', 'en-US.json', 'zh-CN.json', 'ja-JP.json', 'ko-KR.json'];

const fullUpdates = {
    'zh-TW.json': {
        'modals.add_device': '新增設備',
        'modals.bulk_add': '批量新增設備',
        'modals.edit_device': '編輯設備',
        'devices.no_data': '目前沒有設備資料',
        'devices.status_online': '線上',
        'devices.status_offline': '離線',
        'devices.type_router': '路由器',
        'devices.type_switch': '交換器',
        'devices.type_ap': '無線存取點 (AP)',
        'devices.type_firewall': '防火牆',
        'devices.type_server': '伺服器',
        'devices.type_codec': '編解碼器 (Codec)',
        'devices.type_ipcam': '攝影機 (IPCAM)',
        'devices.type_video_wall': '電視牆控制器',
        'devices.type_other': '其他'
    },
    'en-US.json': {
        'modals.add_device': 'Add Device',
        'modals.bulk_add': 'Bulk Add Devices',
        'modals.edit_device': 'Edit Device',
        'devices.no_data': 'No device data found',
        'devices.status_online': 'Online',
        'devices.status_offline': 'Offline',
        'devices.type_router': 'Router',
        'devices.type_switch': 'Switch',
        'devices.type_ap': 'Access Point (AP)',
        'devices.type_firewall': 'Firewall',
        'devices.type_server': 'Server',
        'devices.type_codec': 'Codec',
        'devices.type_ipcam': 'IP Camera',
        'devices.type_video_wall': 'Video Wall Controller',
        'devices.type_other': 'Other'
    },
    'zh-CN.json': {
        'modals.add_device': '新增设备',
        'modals.bulk_add': '批量新增设备',
        'modals.edit_device': '编辑设备',
        'devices.no_data': '目前没有设备数据',
        'devices.status_online': '在线',
        'devices.status_offline': '离线',
        'devices.type_router': '路由器',
        'devices.type_switch': '交换机',
        'devices.type_ap': '无线访问点 (AP)',
        'devices.type_firewall': '防火墙',
        'devices.type_server': '服务器',
        'devices.type_codec': '编解码器 (Codec)',
        'devices.type_ipcam': '摄像机 (IPCAM)',
        'devices.type_video_wall': '电视墙控制器',
        'devices.type_other': '其他'
    },
    'ja-JP.json': {
        'modals.add_device': 'デバイス追加',
        'modals.bulk_add': '一括デバイス追加',
        'modals.edit_device': 'デバイス編集',
        'devices.no_data': 'デバイスデータがありません',
        'devices.status_online': 'オンライン',
        'devices.status_offline': 'オフライン',
        'devices.type_router': 'ルーター',
        'devices.type_switch': 'スイッチ',
        'devices.type_ap': 'アクセスポイント (AP)',
        'devices.type_firewall': 'ファイアウォール',
        'devices.type_server': 'サーバー',
        'devices.type_codec': 'コーデック (Codec)',
        'devices.type_ipcam': 'カメラ (IPCAM)',
        'devices.type_video_wall': 'ビデオウォールコントローラ',
        'devices.type_other': 'その他'
    },
    'ko-KR.json': {
        'modals.add_device': '장치 추가',
        'modals.bulk_add': '일괄 장치 추가',
        'modals.edit_device': '장치 편집',
        'devices.no_data': '장치 데이터가 없습니다',
        'devices.status_online': '온라인',
        'devices.status_offline': '오프라인',
        'devices.type_router': '라우터',
        'devices.type_switch': '스위치',
        'devices.type_ap': '액세스 포인트 (AP)',
        'devices.type_firewall': '방화벽',
        'devices.type_server': '서버',
        'devices.type_codec': '코덱 (Codec)',
        'devices.type_ipcam': '카메라 (IPCAM)',
        'devices.type_video_wall': '비디오 월 컨트롤러',
        'devices.type_other': '기타'
    }
};

languages.forEach(lang => {
    const filePath = path.join(i18nDir, lang);
    if (fs.existsSync(filePath)) {
        let data = JSON.parse(fs.readFileSync(filePath, 'utf8'));
        const updates = fullUpdates[lang];

        Object.keys(updates).forEach(keyPath => {
            const keys = keyPath.split('.');
            let current = data;
            for (let i = 0; i < keys.length - 1; i++) {
                if (!current[keys[i]]) current[keys[i]] = {};
                current = current[keys[i]];
            }
            current[keys[keys.length - 1]] = updates[keyPath];
        });

        fs.writeFileSync(filePath, JSON.stringify(data, null, 4), 'utf8');
        console.log(`Deep Sync: ${lang}`);
    }
});
