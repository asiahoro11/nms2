const fs = require('fs');
const path = require('path');

const i18nDir = 'frontend/i18n';
const languages = ['zh-TW.json', 'en-US.json', 'zh-CN.json', 'ja-JP.json', 'ko-KR.json'];

const modalTitles = {
    'zh-TW.json': {
        'modals.add_device': '新增設備',
        'modals.bulk_add': '批量新增設備',
        'modals.edit_device': '編輯設備'
    },
    'en-US.json': {
        'modals.add_device': 'Add Device',
        'modals.bulk_add': 'Bulk Add Devices',
        'modals.edit_device': 'Edit Device'
    },
    'zh-CN.json': {
        'modals.add_device': '新增设备',
        'modals.bulk_add': '批量新增设备',
        'modals.edit_device': '编辑设备'
    },
    'ja-JP.json': {
        'modals.add_device': 'デバイス追加',
        'modals.bulk_add': '一括デバイス追加',
        'modals.edit_device': 'デバイス編集'
    },
    'ko-KR.json': {
        'modals.add_device': '장치 추가',
        'modals.bulk_add': '일괄 장치 추가',
        'modals.edit_device': '장치 편집'
    }
};

languages.forEach(lang => {
    const filePath = path.join(i18nDir, lang);
    if (fs.existsSync(filePath)) {
        let data = JSON.parse(fs.readFileSync(filePath, 'utf8'));

        // 建立獨立的 modals 區塊
        data.modals = modalTitles[lang].modals || {};
        const titles = modalTitles[lang];
        Object.keys(titles).forEach(k => {
            const keys = k.split('.');
            if (!data[keys[0]]) data[keys[0]] = {};
            data[keys[0]][keys[1]] = titles[k];
        });

        fs.writeFileSync(filePath, JSON.stringify(data, null, 4), 'utf8');
        console.log(`Verified ${lang}`);
    }
});
