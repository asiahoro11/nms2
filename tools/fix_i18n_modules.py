import json
import os

i18n_dir = r"C:\Users\HP\.gemini\antigravity\playground\azure-universe\nms_sync\frontend\i18n"
files = ["en-US.json", "ja-JP.json", "ko-KR.json", "zh-CN.json", "zh-TW.json"]

new_keys = {
    "en-US.json": {
        "login": {"forgot_password": "Forgot Password?"},
        "admin": {
            "modules": {
                "title": "Module Management",
                "desc": "Enable or disable integrated advanced modules. Menu items will be hidden when disabled.",
                "camera_viewer": "Camera Viewer Module",
                "camera_viewer_hint": "Enabling this shows the camera viewer in navigation (requires license).",
                "alerts": "Alert Notification Function",
                "alerts_hint": "Globally enable or disable system alert notifications.",
                "available_channels": "Available Alert Channels"
            }
        }
    },
    "zh-TW.json": {
        "login": {"forgot_password": "忘記密碼？"},
        "admin": {
            "modules": {
                "title": "功能模組管理",
                "desc": "開啟或關閉系統整合之進階功能模組。停用後相關選單將會隱藏。",
                "camera_viewer": "攝影機監控模組",
                "camera_viewer_hint": "啟用後導覽列將出現攝影機監控選項 (需對應授權)。",
                "alerts": "告警通知功能",
                "alerts_hint": "全域開啟或關閉系統告警發送功能。",
                "available_channels": "開放使用的告警管道"
            }
        }
    },
    "zh-CN.json": {
        "login": {"forgot_password": "忘记密码？"},
        "admin": {
            "modules": {
                "title": "功能模块管理",
                "desc": "开启或关闭系统集成的进阶功能模块。停用后相关菜单将会隐藏。",
                "camera_viewer": "摄影机监控模块",
                "camera_viewer_hint": "启用后导航栏将出现摄影机监控选项 (需对应授权)。",
                "alerts": "告警通知功能",
                "alerts_hint": "全局开启或关闭系统告警发送功能。",
                "available_channels": "开放使用的告警管道"
            }
        }
    },
    "ja-JP.json": {
        "login": {"forgot_password": "パスワードを忘れた場合"},
        "admin": {
            "modules": {
                "title": "機能モジュール管理",
                "desc": "統合された高度な機能モジュールを有効または無効にします。無効にすると、関連メニューが非表示になります。",
                "camera_viewer": "カメラ監視モジュール",
                "camera_viewer_hint": "有効にするとナビゲーションにカメラ監視が表示されます (ライセンスが必要)。",
                "alerts": "アラート通知機能",
                "alerts_hint": "システムアラート通知をグローバルに有効または無効にします。",
                "available_channels": "利用可能なアラートチャネル"
            }
        }
    },
    "ko-KR.json": {
        "login": {"forgot_password": "비밀번호를 잊으셨나요?"},
        "admin": {
            "modules": {
                "title": "기능 모듈 관리",
                "desc": "통합된 고급 기능 모듈을 활성화하거나 비활성화합니다. 비활성화하면 관련 메뉴가 숨겨집니다.",
                "camera_viewer": "카메라 모니터링 모듈",
                "camera_viewer_hint": "활성화하면 내비게이션에 카메라 모니터링이 표시됩니다 (라이선스 필요).",
                "alerts": "알람 알림 기능",
                "alerts_hint": "시스템 알람 알림을 전역적으로 활성화하거나 비활성화합니다.",
                "available_channels": "사용 가능한 알람 채널"
            }
        }
    }
}

def merge(target, source):
    for key, value in source.items():
        if isinstance(value, dict) and key in target and isinstance(target[key], dict):
            merge(target[key], value)
        else:
            target[key] = value

for filename in files:
    path = os.path.join(i18n_dir, filename)
    if os.path.exists(path):
        with open(path, 'r', encoding='utf-8') as f:
            data = json.load(f)
        
        merge(data, new_keys[filename])
        
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, ensure_ascii=False, indent=4)
        print(f"Updated {filename}")
    else:
        print(f"Skipped {filename} (not found)")
