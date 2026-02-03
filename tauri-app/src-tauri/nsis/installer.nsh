; ZimaOS Echo - Custom NSIS Installer Hooks
; Modern simplified installation flow

; Custom language strings for all 27 languages
LangString LaunchApp ${LANG_ENGLISH} "Launch ZimaOS Echo"
LangString LaunchApp ${LANG_SIMPCHINESE} "启动 ZimaOS Echo"
LangString LaunchApp ${LANG_TRADCHINESE} "啟動 ZimaOS Echo"
LangString LaunchApp ${LANG_JAPANESE} "ZimaOS Echo を起動"
LangString LaunchApp ${LANG_KOREAN} "ZimaOS Echo 실행"
LangString LaunchApp ${LANG_GERMAN} "ZimaOS Echo starten"
LangString LaunchApp ${LANG_FRENCH} "Lancer ZimaOS Echo"
LangString LaunchApp ${LANG_SPANISH} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_SPANISHINTERNATIONAL} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_PORTUGUESE} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_PORTUGUESEBR} "Iniciar ZimaOS Echo"
LangString LaunchApp ${LANG_ITALIAN} "Avvia ZimaOS Echo"
LangString LaunchApp ${LANG_DUTCH} "ZimaOS Echo starten"
LangString LaunchApp ${LANG_RUSSIAN} "Запустить ZimaOS Echo"
LangString LaunchApp ${LANG_POLISH} "Uruchom ZimaOS Echo"
LangString LaunchApp ${LANG_UKRAINIAN} "Запустити ZimaOS Echo"
LangString LaunchApp ${LANG_CZECH} "Spustit ZimaOS Echo"
LangString LaunchApp ${LANG_SLOVAK} "Spustiť ZimaOS Echo"
LangString LaunchApp ${LANG_HUNGARIAN} "ZimaOS Echo indítása"
LangString LaunchApp ${LANG_ROMANIAN} "Lansați ZimaOS Echo"
LangString LaunchApp ${LANG_BULGARIAN} "Стартирайте ZimaOS Echo"
LangString LaunchApp ${LANG_TURKISH} "ZimaOS Echo'yu Başlat"
LangString LaunchApp ${LANG_ARABIC} "تشغيل ZimaOS Echo"
LangString LaunchApp ${LANG_HEBREW} "הפעל את ZimaOS Echo"
LangString LaunchApp ${LANG_THAI} "เปิด ZimaOS Echo"
LangString LaunchApp ${LANG_VIETNAMESE} "Khởi chạy ZimaOS Echo"
LangString LaunchApp ${LANG_INDONESIAN} "Jalankan ZimaOS Echo"

; Installing message
LangString InstallingMsg ${LANG_ENGLISH} "Installing ZimaOS Echo..."
LangString InstallingMsg ${LANG_SIMPCHINESE} "正在安装 ZimaOS Echo..."
LangString InstallingMsg ${LANG_TRADCHINESE} "正在安裝 ZimaOS Echo..."
LangString InstallingMsg ${LANG_JAPANESE} "ZimaOS Echo をインストール中..."
LangString InstallingMsg ${LANG_KOREAN} "ZimaOS Echo 설치 중..."
LangString InstallingMsg ${LANG_GERMAN} "ZimaOS Echo wird installiert..."
LangString InstallingMsg ${LANG_FRENCH} "Installation de ZimaOS Echo..."
LangString InstallingMsg ${LANG_SPANISH} "Instalando ZimaOS Echo..."
LangString InstallingMsg ${LANG_SPANISHINTERNATIONAL} "Instalando ZimaOS Echo..."
LangString InstallingMsg ${LANG_PORTUGUESE} "Instalando ZimaOS Echo..."
LangString InstallingMsg ${LANG_PORTUGUESEBR} "Instalando ZimaOS Echo..."
LangString InstallingMsg ${LANG_ITALIAN} "Installazione di ZimaOS Echo..."
LangString InstallingMsg ${LANG_DUTCH} "ZimaOS Echo installeren..."
LangString InstallingMsg ${LANG_RUSSIAN} "Установка ZimaOS Echo..."
LangString InstallingMsg ${LANG_POLISH} "Instalowanie ZimaOS Echo..."
LangString InstallingMsg ${LANG_UKRAINIAN} "Встановлення ZimaOS Echo..."
LangString InstallingMsg ${LANG_CZECH} "Instalace ZimaOS Echo..."
LangString InstallingMsg ${LANG_SLOVAK} "Inštalácia ZimaOS Echo..."
LangString InstallingMsg ${LANG_HUNGARIAN} "ZimaOS Echo telepítése..."
LangString InstallingMsg ${LANG_ROMANIAN} "Se instalează ZimaOS Echo..."
LangString InstallingMsg ${LANG_BULGARIAN} "Инсталиране на ZimaOS Echo..."
LangString InstallingMsg ${LANG_TURKISH} "ZimaOS Echo yükleniyor..."
LangString InstallingMsg ${LANG_ARABIC} "جاري تثبيت ZimaOS Echo..."
LangString InstallingMsg ${LANG_HEBREW} "מתקין את ZimaOS Echo..."
LangString InstallingMsg ${LANG_THAI} "กำลังติดตั้ง ZimaOS Echo..."
LangString InstallingMsg ${LANG_VIETNAMESE} "Đang cài đặt ZimaOS Echo..."
LangString InstallingMsg ${LANG_INDONESIAN} "Menginstal ZimaOS Echo..."

; Pre-install hook
!macro NSIS_HOOK_PREINSTALL
  DetailPrint "$(InstallingMsg)"
!macroend

; Post-install hook
!macro NSIS_HOOK_POSTINSTALL
  ; Installation complete
!macroend
