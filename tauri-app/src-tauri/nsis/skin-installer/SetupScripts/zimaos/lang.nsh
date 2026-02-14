; ZimaOS Blue Installer - Language Strings
; Supports 27 languages

Var LangInstallTitle
Var LangUninstallTitle
Var LangOneClickInstall
Var LangStartUsing
Var LangInstallPath
Var LangRequiredSpace
Var LangRemainingSpace
Var LangLicenseAgreement
Var LangIAgree
Var LangRunning
Var LangExitConfirm
Var LangPathInvalid
Var LangDiskSpaceLow
Var LangUninstall
Var LangUninstallConfirm
Var LangTip

Function SetLanguageStrings
  ; Get system language
  System::Call 'kernel32::GetUserDefaultUILanguage() i .r0'

  ; Default to English
  StrCpy $LangInstallTitle "${PRODUCT_NAME} Setup"
  StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
  StrCpy $LangOneClickInstall "Install"
  StrCpy $LangStartUsing "Start"
  StrCpy $LangInstallPath "Install Path:"
  StrCpy $LangRequiredSpace "Required: 100MB"
  StrCpy $LangRemainingSpace "Free Space:"
  StrCpy $LangLicenseAgreement "License Agreement"
  StrCpy $LangIAgree "I have read and agree to"
  StrCpy $LangRunning "${PRODUCT_NAME} is running. Please close it first!"
  StrCpy $LangExitConfirm "Installation not complete. Are you sure you want to exit?"
  StrCpy $LangPathInvalid "Invalid path"
  StrCpy $LangDiskSpaceLow "Not enough disk space!"
  StrCpy $LangUninstall "Uninstall"
  StrCpy $LangUninstallConfirm "Are you sure you want to uninstall ${PRODUCT_NAME}?"
  StrCpy $LangTip "Notice"

  ; Chinese Simplified (0x0804)
  IntCmp $0 2052 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} 安装程序"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 卸载程序"
    StrCpy $LangOneClickInstall "一键安装"
    StrCpy $LangStartUsing "开始使用"
    StrCpy $LangInstallPath "安装路径："
    StrCpy $LangRequiredSpace "所需空间：100MB"
    StrCpy $LangRemainingSpace "剩余空间："
    StrCpy $LangLicenseAgreement "软件服务条款"
    StrCpy $LangIAgree "我已经阅读并认可"
    StrCpy $LangRunning "${PRODUCT_NAME} 正在运行，请退出后重试!"
    StrCpy $LangExitConfirm "安装尚未完成，您确定退出安装么？"
    StrCpy $LangPathInvalid "路径非法"
    StrCpy $LangDiskSpaceLow "目标磁盘空间不足！"
    StrCpy $LangUninstall "卸载"
    StrCpy $LangUninstallConfirm "确定要卸载 ${PRODUCT_NAME} 吗？"
    StrCpy $LangTip "提示"
    Goto LangDone

  ; Chinese Traditional (0x0404)
  IntCmp $0 1028 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} 安裝程式"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 解除安裝"
    StrCpy $LangOneClickInstall "一鍵安裝"
    StrCpy $LangStartUsing "開始使用"
    StrCpy $LangInstallPath "安裝路徑："
    StrCpy $LangRequiredSpace "所需空間：100MB"
    StrCpy $LangRemainingSpace "剩餘空間："
    StrCpy $LangLicenseAgreement "軟體服務條款"
    StrCpy $LangIAgree "我已經閱讀並認可"
    StrCpy $LangRunning "${PRODUCT_NAME} 正在運行，請退出後重試!"
    StrCpy $LangExitConfirm "安裝尚未完成，您確定退出安裝嗎？"
    StrCpy $LangPathInvalid "路徑無效"
    StrCpy $LangDiskSpaceLow "目標磁碟空間不足！"
    StrCpy $LangUninstall "解除安裝"
    StrCpy $LangUninstallConfirm "確定要解除安裝 ${PRODUCT_NAME} 嗎？"
    StrCpy $LangTip "提示"
    Goto LangDone

  ; Japanese (0x0411)
  IntCmp $0 1041 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} セットアップ"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} アンインストール"
    StrCpy $LangOneClickInstall "インストール"
    StrCpy $LangStartUsing "開始"
    StrCpy $LangInstallPath "インストール先："
    StrCpy $LangRequiredSpace "必要容量：100MB"
    StrCpy $LangRemainingSpace "空き容量："
    StrCpy $LangLicenseAgreement "使用許諾契約"
    StrCpy $LangIAgree "同意します"
    StrCpy $LangRunning "${PRODUCT_NAME} が実行中です。終了してから再試行してください。"
    StrCpy $LangExitConfirm "インストールが完了していません。終了しますか？"
    StrCpy $LangPathInvalid "無効なパス"
    StrCpy $LangDiskSpaceLow "ディスク容量が不足しています！"
    StrCpy $LangUninstall "アンインストール"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} をアンインストールしますか？"
    StrCpy $LangTip "通知"
    Goto LangDone

  ; Korean (0x0412)
  IntCmp $0 1042 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} 설치"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 제거"
    StrCpy $LangOneClickInstall "설치"
    StrCpy $LangStartUsing "시작"
    StrCpy $LangInstallPath "설치 경로:"
    StrCpy $LangRequiredSpace "필요 공간: 100MB"
    StrCpy $LangRemainingSpace "남은 공간:"
    StrCpy $LangLicenseAgreement "라이선스 계약"
    StrCpy $LangIAgree "동의합니다"
    StrCpy $LangRunning "${PRODUCT_NAME}이(가) 실행 중입니다. 종료 후 다시 시도하세요."
    StrCpy $LangExitConfirm "설치가 완료되지 않았습니다. 종료하시겠습니까?"
    StrCpy $LangPathInvalid "잘못된 경로"
    StrCpy $LangDiskSpaceLow "디스크 공간이 부족합니다!"
    StrCpy $LangUninstall "제거"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME}을(를) 제거하시겠습니까?"
    StrCpy $LangTip "알림"
    Goto LangDone

  ; German (0x0407)
  IntCmp $0 1031 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Installation"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Deinstallation"
    StrCpy $LangOneClickInstall "Installieren"
    StrCpy $LangStartUsing "Starten"
    StrCpy $LangInstallPath "Installationspfad:"
    StrCpy $LangRequiredSpace "Erforderlich: 100MB"
    StrCpy $LangRemainingSpace "Freier Speicher:"
    StrCpy $LangLicenseAgreement "Lizenzvereinbarung"
    StrCpy $LangIAgree "Ich stimme zu"
    StrCpy $LangRunning "${PRODUCT_NAME} wird ausgefuhrt. Bitte beenden Sie es zuerst!"
    StrCpy $LangExitConfirm "Installation nicht abgeschlossen. Wirklich beenden?"
    StrCpy $LangPathInvalid "Ungultiger Pfad"
    StrCpy $LangDiskSpaceLow "Nicht genugend Speicherplatz!"
    StrCpy $LangUninstall "Deinstallieren"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} wirklich deinstallieren?"
    StrCpy $LangTip "Hinweis"
    Goto LangDone

  ; French (0x040C)
  IntCmp $0 1036 0 +17 +17
    StrCpy $LangInstallTitle "Installation de ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Desinstallation de ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Installer"
    StrCpy $LangStartUsing "Demarrer"
    StrCpy $LangInstallPath "Chemin d'installation:"
    StrCpy $LangRequiredSpace "Requis: 100MB"
    StrCpy $LangRemainingSpace "Espace libre:"
    StrCpy $LangLicenseAgreement "Contrat de licence"
    StrCpy $LangIAgree "J'accepte"
    StrCpy $LangRunning "${PRODUCT_NAME} est en cours d'execution. Veuillez le fermer!"
    StrCpy $LangExitConfirm "Installation incomplete. Voulez-vous vraiment quitter?"
    StrCpy $LangPathInvalid "Chemin invalide"
    StrCpy $LangDiskSpaceLow "Espace disque insuffisant!"
    StrCpy $LangUninstall "Desinstaller"
    StrCpy $LangUninstallConfirm "Voulez-vous vraiment desinstaller ${PRODUCT_NAME}?"
    StrCpy $LangTip "Information"
    Goto LangDone

  ; Spanish (0x0C0A)
  IntCmp $0 3082 0 +17 +17
    StrCpy $LangInstallTitle "Instalacion de ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Desinstalacion de ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instalar"
    StrCpy $LangStartUsing "Iniciar"
    StrCpy $LangInstallPath "Ruta de instalacion:"
    StrCpy $LangRequiredSpace "Requerido: 100MB"
    StrCpy $LangRemainingSpace "Espacio libre:"
    StrCpy $LangLicenseAgreement "Acuerdo de licencia"
    StrCpy $LangIAgree "Acepto"
    StrCpy $LangRunning "${PRODUCT_NAME} esta en ejecucion. Cierrelo primero!"
    StrCpy $LangExitConfirm "Instalacion incompleta. Desea salir?"
    StrCpy $LangPathInvalid "Ruta invalida"
    StrCpy $LangDiskSpaceLow "Espacio en disco insuficiente!"
    StrCpy $LangUninstall "Desinstalar"
    StrCpy $LangUninstallConfirm "Desea desinstalar ${PRODUCT_NAME}?"
    StrCpy $LangTip "Aviso"
    Goto LangDone

  ; Portuguese (0x0416)
  IntCmp $0 1046 0 +17 +17
    StrCpy $LangInstallTitle "Instalacao do ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Desinstalacao do ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instalar"
    StrCpy $LangStartUsing "Iniciar"
    StrCpy $LangInstallPath "Caminho de instalacao:"
    StrCpy $LangRequiredSpace "Necessario: 100MB"
    StrCpy $LangRemainingSpace "Espaco livre:"
    StrCpy $LangLicenseAgreement "Contrato de licenca"
    StrCpy $LangIAgree "Eu concordo"
    StrCpy $LangRunning "${PRODUCT_NAME} esta em execucao. Feche-o primeiro!"
    StrCpy $LangExitConfirm "Instalacao incompleta. Deseja sair?"
    StrCpy $LangPathInvalid "Caminho invalido"
    StrCpy $LangDiskSpaceLow "Espaco em disco insuficiente!"
    StrCpy $LangUninstall "Desinstalar"
    StrCpy $LangUninstallConfirm "Deseja desinstalar o ${PRODUCT_NAME}?"
    StrCpy $LangTip "Aviso"
    Goto LangDone

  ; Russian (0x0419)
  IntCmp $0 1049 0 +17 +17
    StrCpy $LangInstallTitle "Ustanovka ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Udalenie ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Ustanovit"
    StrCpy $LangStartUsing "Zapustit"
    StrCpy $LangInstallPath "Put ustanovki:"
    StrCpy $LangRequiredSpace "Trebuetsya: 100MB"
    StrCpy $LangRemainingSpace "Svobodno:"
    StrCpy $LangLicenseAgreement "Licenzionnoe soglashenie"
    StrCpy $LangIAgree "Ya soglashen"
    StrCpy $LangRunning "${PRODUCT_NAME} zapushen. Zakroyte ego!"
    StrCpy $LangExitConfirm "Ustanovka ne zavershena. Vyyti?"
    StrCpy $LangPathInvalid "Nevernyy put"
    StrCpy $LangDiskSpaceLow "Nedostatochno mesta na diske!"
    StrCpy $LangUninstall "Udalit"
    StrCpy $LangUninstallConfirm "Udalit ${PRODUCT_NAME}?"
    StrCpy $LangTip "Uvedomlenie"
    Goto LangDone

  ; Italian (0x0410)
  IntCmp $0 1040 0 +17 +17
    StrCpy $LangInstallTitle "Installazione di ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Disinstallazione di ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Installa"
    StrCpy $LangStartUsing "Avvia"
    StrCpy $LangInstallPath "Percorso di installazione:"
    StrCpy $LangRequiredSpace "Richiesto: 100MB"
    StrCpy $LangRemainingSpace "Spazio libero:"
    StrCpy $LangLicenseAgreement "Contratto di licenza"
    StrCpy $LangIAgree "Accetto"
    StrCpy $LangRunning "${PRODUCT_NAME} e in esecuzione. Chiuderlo prima!"
    StrCpy $LangExitConfirm "Installazione incompleta. Uscire?"
    StrCpy $LangPathInvalid "Percorso non valido"
    StrCpy $LangDiskSpaceLow "Spazio su disco insufficiente!"
    StrCpy $LangUninstall "Disinstalla"
    StrCpy $LangUninstallConfirm "Disinstallare ${PRODUCT_NAME}?"
    StrCpy $LangTip "Avviso"
    Goto LangDone

  ; Dutch (0x0413)
  IntCmp $0 1043 0 +17 +17
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Installatie"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Verwijderen"
    StrCpy $LangOneClickInstall "Installeren"
    StrCpy $LangStartUsing "Starten"
    StrCpy $LangInstallPath "Installatiepad:"
    StrCpy $LangRequiredSpace "Vereist: 100MB"
    StrCpy $LangRemainingSpace "Vrije ruimte:"
    StrCpy $LangLicenseAgreement "Licentieovereenkomst"
    StrCpy $LangIAgree "Ik ga akkoord"
    StrCpy $LangRunning "${PRODUCT_NAME} is actief. Sluit het eerst!"
    StrCpy $LangExitConfirm "Installatie niet voltooid. Afsluiten?"
    StrCpy $LangPathInvalid "Ongeldig pad"
    StrCpy $LangDiskSpaceLow "Onvoldoende schijfruimte!"
    StrCpy $LangUninstall "Verwijderen"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} verwijderen?"
    StrCpy $LangTip "Melding"
    Goto LangDone

LangDone:
FunctionEnd

Function un.SetLanguageStrings
  System::Call 'kernel32::GetUserDefaultUILanguage() i .r0'

  ; Default English
  StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
  StrCpy $LangUninstallConfirm "Are you sure you want to uninstall ${PRODUCT_NAME}?"
  StrCpy $LangRunning "${PRODUCT_NAME} is running. Please close it first!"
  StrCpy $LangTip "Notice"

  ; Chinese Simplified
  IntCmp $0 2052 0 +5 +5
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 卸载程序"
    StrCpy $LangUninstallConfirm "确定要卸载 ${PRODUCT_NAME} 吗？"
    StrCpy $LangRunning "${PRODUCT_NAME} 正在运行，请退出后重试!"
    StrCpy $LangTip "提示"

FunctionEnd
