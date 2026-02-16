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
Var LangConfirm
Var LangCancel

Function SetLanguageStrings
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
  StrCpy $LangConfirm "OK"
  StrCpy $LangCancel "Cancel"

  ; Chinese Simplified (0x0804)
  IntCmp $0 2052 0 +21 +21
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
    StrCpy $LangConfirm "确 定"
    StrCpy $LangCancel "取 消"
    Goto LangDone

  ; Chinese Traditional (0x0404)
  IntCmp $0 1028 0 +21 +21
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
    StrCpy $LangConfirm "確 定"
    StrCpy $LangCancel "取 消"
    Goto LangDone

  ; Japanese (0x0411)
  IntCmp $0 1041 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "キャンセル"
    Goto LangDone

  ; Korean (0x0412)
  IntCmp $0 1042 0 +21 +21
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
    StrCpy $LangConfirm "확인"
    StrCpy $LangCancel "취소"
    Goto LangDone

  ; German (0x0407)
  IntCmp $0 1031 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Abbrechen"
    Goto LangDone

  ; French (0x040C)
  IntCmp $0 1036 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuler"
    Goto LangDone

  ; Spanish (0x0C0A)
  IntCmp $0 3082 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto LangDone

  ; Portuguese Brazil (0x0416)
  IntCmp $0 1046 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto LangDone

  ; Portuguese Portugal (0x0816)
  IntCmp $0 2070 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto LangDone

  ; Russian (0x0419)
  IntCmp $0 1049 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Otmena"
    Goto LangDone

  ; Italian (0x0410)
  IntCmp $0 1040 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annulla"
    Goto LangDone

  ; Dutch (0x0413)
  IntCmp $0 1043 0 +21 +21
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
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuleren"
    Goto LangDone

  ; Polish (0x0415)
  IntCmp $0 1045 0 +21 +21
    StrCpy $LangInstallTitle "Instalacja ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Deinstalacja ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Zainstaluj"
    StrCpy $LangStartUsing "Uruchom"
    StrCpy $LangInstallPath "Sciezka instalacji:"
    StrCpy $LangRequiredSpace "Wymagane: 100MB"
    StrCpy $LangRemainingSpace "Wolne miejsce:"
    StrCpy $LangLicenseAgreement "Umowa licencyjna"
    StrCpy $LangIAgree "Akceptuje"
    StrCpy $LangRunning "${PRODUCT_NAME} jest uruchomiony. Zamknij go najpierw!"
    StrCpy $LangExitConfirm "Instalacja niekompletna. Czy na pewno chcesz wyjsc?"
    StrCpy $LangPathInvalid "Nieprawidlowa sciezka"
    StrCpy $LangDiskSpaceLow "Za malo miejsca na dysku!"
    StrCpy $LangUninstall "Odinstaluj"
    StrCpy $LangUninstallConfirm "Czy na pewno chcesz odinstalowac ${PRODUCT_NAME}?"
    StrCpy $LangTip "Informacja"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Anuluj"
    Goto LangDone

  ; Czech (0x0405)
  IntCmp $0 1029 0 +21 +21
    StrCpy $LangInstallTitle "Instalace ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Odinstalace ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instalovat"
    StrCpy $LangStartUsing "Spustit"
    StrCpy $LangInstallPath "Cesta instalace:"
    StrCpy $LangRequiredSpace "Pozadovano: 100MB"
    StrCpy $LangRemainingSpace "Volne misto:"
    StrCpy $LangLicenseAgreement "Licencni smlouva"
    StrCpy $LangIAgree "Souhlasim"
    StrCpy $LangRunning "${PRODUCT_NAME} je spusten. Nejprve ho ukoncete!"
    StrCpy $LangExitConfirm "Instalace neni dokoncena. Opravdu chcete ukoncit?"
    StrCpy $LangPathInvalid "Neplatna cesta"
    StrCpy $LangDiskSpaceLow "Nedostatek mista na disku!"
    StrCpy $LangUninstall "Odinstalovat"
    StrCpy $LangUninstallConfirm "Opravdu chcete odinstalovat ${PRODUCT_NAME}?"
    StrCpy $LangTip "Upozorneni"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Zrusit"
    Goto LangDone

  ; Slovak (0x041B)
  IntCmp $0 1051 0 +21 +21
    StrCpy $LangInstallTitle "Instalacia ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Odinstalovanie ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instalovat"
    StrCpy $LangStartUsing "Spustit"
    StrCpy $LangInstallPath "Cesta instalacie:"
    StrCpy $LangRequiredSpace "Pozadovane: 100MB"
    StrCpy $LangRemainingSpace "Volne miesto:"
    StrCpy $LangLicenseAgreement "Licencna zmluva"
    StrCpy $LangIAgree "Suhlasim"
    StrCpy $LangRunning "${PRODUCT_NAME} je spusteny. Najprv ho ukoncite!"
    StrCpy $LangExitConfirm "Instalacia nie je dokoncena. Naozaj chcete ukoncit?"
    StrCpy $LangPathInvalid "Neplatna cesta"
    StrCpy $LangDiskSpaceLow "Nedostatok miesta na disku!"
    StrCpy $LangUninstall "Odinstalovanie"
    StrCpy $LangUninstallConfirm "Naozaj chcete odinstalovanie ${PRODUCT_NAME}?"
    StrCpy $LangTip "Upozornenie"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Zrusit"
    Goto LangDone

  ; Hungarian (0x040E)
  IntCmp $0 1038 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Telepites"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Eltavolitas"
    StrCpy $LangOneClickInstall "Telepites"
    StrCpy $LangStartUsing "Inditas"
    StrCpy $LangInstallPath "Telepitesi utvonal:"
    StrCpy $LangRequiredSpace "Szukseges: 100MB"
    StrCpy $LangRemainingSpace "Szabad hely:"
    StrCpy $LangLicenseAgreement "Licencszerzodes"
    StrCpy $LangIAgree "Elfogadom"
    StrCpy $LangRunning "${PRODUCT_NAME} fut. Kerem, zarje be eloszor!"
    StrCpy $LangExitConfirm "A telepites nem fejezodott be. Biztosan ki akar lepni?"
    StrCpy $LangPathInvalid "Ervenytelen utvonal"
    StrCpy $LangDiskSpaceLow "Nincs eleg lemezterulet!"
    StrCpy $LangUninstall "Eltavolitas"
    StrCpy $LangUninstallConfirm "Biztosan el akarja tavolitani a ${PRODUCT_NAME} programot?"
    StrCpy $LangTip "Ertesites"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Megse"
    Goto LangDone

  ; Romanian (0x0418)
  IntCmp $0 1048 0 +21 +21
    StrCpy $LangInstallTitle "Instalare ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Dezinstalare ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instaleaza"
    StrCpy $LangStartUsing "Porneste"
    StrCpy $LangInstallPath "Cale de instalare:"
    StrCpy $LangRequiredSpace "Necesar: 100MB"
    StrCpy $LangRemainingSpace "Spatiu liber:"
    StrCpy $LangLicenseAgreement "Acord de licenta"
    StrCpy $LangIAgree "Sunt de acord"
    StrCpy $LangRunning "${PRODUCT_NAME} ruleaza. Inchideti-l mai intai!"
    StrCpy $LangExitConfirm "Instalarea nu este completa. Sigur doriti sa iesiti?"
    StrCpy $LangPathInvalid "Cale invalida"
    StrCpy $LangDiskSpaceLow "Spatiu insuficient pe disc!"
    StrCpy $LangUninstall "Dezinstaleaza"
    StrCpy $LangUninstallConfirm "Sigur doriti sa dezinstalati ${PRODUCT_NAME}?"
    StrCpy $LangTip "Notificare"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Anuleaza"
    Goto LangDone

  ; Swedish (0x041D)
  IntCmp $0 1053 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Installation"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Avinstallation"
    StrCpy $LangOneClickInstall "Installera"
    StrCpy $LangStartUsing "Starta"
    StrCpy $LangInstallPath "Installationssokvag:"
    StrCpy $LangRequiredSpace "Kravs: 100MB"
    StrCpy $LangRemainingSpace "Ledigt utrymme:"
    StrCpy $LangLicenseAgreement "Licensavtal"
    StrCpy $LangIAgree "Jag godkanner"
    StrCpy $LangRunning "${PRODUCT_NAME} kors. Stang det forst!"
    StrCpy $LangExitConfirm "Installationen ar inte klar. Vill du avsluta?"
    StrCpy $LangPathInvalid "Ogiltig sokvag"
    StrCpy $LangDiskSpaceLow "Inte tillrackligt med diskutrymme!"
    StrCpy $LangUninstall "Avinstallera"
    StrCpy $LangUninstallConfirm "Vill du avinstallera ${PRODUCT_NAME}?"
    StrCpy $LangTip "Meddelande"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Avbryt"
    Goto LangDone

  ; Danish (0x0406)
  IntCmp $0 1030 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Installation"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Afinstallation"
    StrCpy $LangOneClickInstall "Installer"
    StrCpy $LangStartUsing "Start"
    StrCpy $LangInstallPath "Installationssti:"
    StrCpy $LangRequiredSpace "Pakraevet: 100MB"
    StrCpy $LangRemainingSpace "Ledig plads:"
    StrCpy $LangLicenseAgreement "Licensaftale"
    StrCpy $LangIAgree "Jeg accepterer"
    StrCpy $LangRunning "${PRODUCT_NAME} korer. Luk det forst!"
    StrCpy $LangExitConfirm "Installationen er ikke faerdig. Vil du afslutte?"
    StrCpy $LangPathInvalid "Ugyldig sti"
    StrCpy $LangDiskSpaceLow "Ikke nok diskplads!"
    StrCpy $LangUninstall "Afinstaller"
    StrCpy $LangUninstallConfirm "Vil du afinstallere ${PRODUCT_NAME}?"
    StrCpy $LangTip "Besked"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuller"
    Goto LangDone

  ; Norwegian Bokmal (0x0414)
  IntCmp $0 1044 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Installasjon"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Avinstallasjon"
    StrCpy $LangOneClickInstall "Installer"
    StrCpy $LangStartUsing "Start"
    StrCpy $LangInstallPath "Installasjonssti:"
    StrCpy $LangRequiredSpace "Pakrevd: 100MB"
    StrCpy $LangRemainingSpace "Ledig plass:"
    StrCpy $LangLicenseAgreement "Lisensavtale"
    StrCpy $LangIAgree "Jeg godtar"
    StrCpy $LangRunning "${PRODUCT_NAME} kjorer. Lukk det forst!"
    StrCpy $LangExitConfirm "Installasjonen er ikke fullfort. Vil du avslutte?"
    StrCpy $LangPathInvalid "Ugyldig sti"
    StrCpy $LangDiskSpaceLow "Ikke nok diskplass!"
    StrCpy $LangUninstall "Avinstaller"
    StrCpy $LangUninstallConfirm "Vil du avinstallere ${PRODUCT_NAME}?"
    StrCpy $LangTip "Melding"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Avbryt"
    Goto LangDone

  ; Croatian (0x041A)
  IntCmp $0 1050 0 +21 +21
    StrCpy $LangInstallTitle "Instalacija ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Deinstalacija ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instaliraj"
    StrCpy $LangStartUsing "Pokreni"
    StrCpy $LangInstallPath "Put instalacije:"
    StrCpy $LangRequiredSpace "Potrebno: 100MB"
    StrCpy $LangRemainingSpace "Slobodan prostor:"
    StrCpy $LangLicenseAgreement "Licencni ugovor"
    StrCpy $LangIAgree "Prihvacam"
    StrCpy $LangRunning "${PRODUCT_NAME} je pokrenut. Zatvorite ga prvo!"
    StrCpy $LangExitConfirm "Instalacija nije zavrsena. Zelite li izaci?"
    StrCpy $LangPathInvalid "Nevazeci put"
    StrCpy $LangDiskSpaceLow "Nedovoljno prostora na disku!"
    StrCpy $LangUninstall "Deinstaliraj"
    StrCpy $LangUninstallConfirm "Zelite li deinstalirati ${PRODUCT_NAME}?"
    StrCpy $LangTip "Obavijest"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Odustani"
    Goto LangDone

  ; Greek (0x0408)
  IntCmp $0 1032 0 +21 +21
    StrCpy $LangInstallTitle "Egkatastasi ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Apegkatastasi ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Egkatastasi"
    StrCpy $LangStartUsing "Ekkinisi"
    StrCpy $LangInstallPath "Diadromi egkatastasis:"
    StrCpy $LangRequiredSpace "Apaiteitai: 100MB"
    StrCpy $LangRemainingSpace "Eleftheros choros:"
    StrCpy $LangLicenseAgreement "Symvasi adeias"
    StrCpy $LangIAgree "Symfonoume"
    StrCpy $LangRunning "${PRODUCT_NAME} ekteleite. Kleiste to prota!"
    StrCpy $LangExitConfirm "I egkatastasi den oloklirothike. Thelete na exelthete?"
    StrCpy $LangPathInvalid "Mi egkyri diadromi"
    StrCpy $LangDiskSpaceLow "Den yparchi arketos choros sto disko!"
    StrCpy $LangUninstall "Apegkatastasi"
    StrCpy $LangUninstallConfirm "Thelete na apegkatastisete to ${PRODUCT_NAME}?"
    StrCpy $LangTip "Eidopoiisi"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Akyrosi"
    Goto LangDone

  ; Catalan (0x0403)
  IntCmp $0 1027 0 +21 +21
    StrCpy $LangInstallTitle "Installacio de ${PRODUCT_NAME}"
    StrCpy $LangUninstallTitle "Desinstallacio de ${PRODUCT_NAME}"
    StrCpy $LangOneClickInstall "Instal-lar"
    StrCpy $LangStartUsing "Iniciar"
    StrCpy $LangInstallPath "Cami d'installacio:"
    StrCpy $LangRequiredSpace "Requerit: 100MB"
    StrCpy $LangRemainingSpace "Espai lliure:"
    StrCpy $LangLicenseAgreement "Acord de llicencia"
    StrCpy $LangIAgree "Hi estic d'acord"
    StrCpy $LangRunning "${PRODUCT_NAME} s'esta executant. Tanqueu-lo primer!"
    StrCpy $LangExitConfirm "La installacio no esta completa. Voleu sortir?"
    StrCpy $LangPathInvalid "Cami no valid"
    StrCpy $LangDiskSpaceLow "No hi ha prou espai al disc!"
    StrCpy $LangUninstall "Desinstal-lar"
    StrCpy $LangUninstallConfirm "Voleu desinstal-lar ${PRODUCT_NAME}?"
    StrCpy $LangTip "Avis"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancel-lar"
    Goto LangDone

  ; Irish (0x083C)
  IntCmp $0 2108 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Setup"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
    StrCpy $LangOneClickInstall "Suiteail"
    StrCpy $LangStartUsing "Tosaigh"
    StrCpy $LangInstallPath "Cosain suiteala:"
    StrCpy $LangRequiredSpace "Riachtanach: 100MB"
    StrCpy $LangRemainingSpace "Spas saor:"
    StrCpy $LangLicenseAgreement "Comhaontu ceadunas"
    StrCpy $LangIAgree "Aontaim"
    StrCpy $LangRunning "${PRODUCT_NAME} ag rith. Dun e ar dtus!"
    StrCpy $LangExitConfirm "Nil an suiteail criochnaithe. An bhfuil tu cinnte?"
    StrCpy $LangPathInvalid "Cosain neamhbhaili"
    StrCpy $LangDiskSpaceLow "Gan go leor spas diosca!"
    StrCpy $LangUninstall "Disuiteail"
    StrCpy $LangUninstallConfirm "An bhfuil tu cinnte gur mian leat ${PRODUCT_NAME} a dhisuiteail?"
    StrCpy $LangTip "Fogra"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cealaigh"
    Goto LangDone

  ; Malayalam (0x044C)
  IntCmp $0 1100 0 +21 +21
    StrCpy $LangInstallTitle "${PRODUCT_NAME} Setup"
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
    StrCpy $LangOneClickInstall "Install"
    StrCpy $LangStartUsing "Start"
    StrCpy $LangInstallPath "Install Path:"
    StrCpy $LangRequiredSpace "Required: 100MB"
    StrCpy $LangRemainingSpace "Free Space:"
    StrCpy $LangLicenseAgreement "License Agreement"
    StrCpy $LangIAgree "I agree"
    StrCpy $LangRunning "${PRODUCT_NAME} is running. Please close it first!"
    StrCpy $LangExitConfirm "Installation not complete. Are you sure you want to exit?"
    StrCpy $LangPathInvalid "Invalid path"
    StrCpy $LangDiskSpaceLow "Not enough disk space!"
    StrCpy $LangUninstall "Uninstall"
    StrCpy $LangUninstallConfirm "Are you sure you want to uninstall ${PRODUCT_NAME}?"
    StrCpy $LangTip "Notice"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancel"
    Goto LangDone

LangDone:
FunctionEnd

Function un.SetLanguageStrings
  System::Call 'kernel32::GetUserDefaultUILanguage() i .r0'

  ; Default to English
  StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
  StrCpy $LangUninstallConfirm "Are you sure you want to uninstall ${PRODUCT_NAME}?"
  StrCpy $LangRunning "${PRODUCT_NAME} is running. Please close it first!"
  StrCpy $LangTip "Notice"
  StrCpy $LangConfirm "OK"
  StrCpy $LangCancel "Cancel"

  ; Chinese Simplified (0x0804)
  IntCmp $0 2052 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 卸载程序"
    StrCpy $LangUninstallConfirm "确定要卸载 ${PRODUCT_NAME} 吗？"
    StrCpy $LangRunning "${PRODUCT_NAME} 正在运行，请退出后重试!"
    StrCpy $LangTip "提示"
    StrCpy $LangConfirm "确 定"
    StrCpy $LangCancel "取 消"
    Goto UnLangDone

  ; Chinese Traditional (0x0404)
  IntCmp $0 1028 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 解除安裝"
    StrCpy $LangUninstallConfirm "確定要解除安裝 ${PRODUCT_NAME} 嗎？"
    StrCpy $LangRunning "${PRODUCT_NAME} 正在運行，請退出後重試!"
    StrCpy $LangTip "提示"
    StrCpy $LangConfirm "確 定"
    StrCpy $LangCancel "取 消"
    Goto UnLangDone

  ; Japanese (0x0411)
  IntCmp $0 1041 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} アンインストール"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} をアンインストールしますか？"
    StrCpy $LangRunning "${PRODUCT_NAME} が実行中です。終了してから再試行してください。"
    StrCpy $LangTip "通知"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "キャンセル"
    Goto UnLangDone

  ; Korean (0x0412)
  IntCmp $0 1042 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} 제거"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME}을(를) 제거하시겠습니까?"
    StrCpy $LangRunning "${PRODUCT_NAME}이(가) 실행 중입니다. 종료 후 다시 시도하세요."
    StrCpy $LangTip "알림"
    StrCpy $LangConfirm "확인"
    StrCpy $LangCancel "취소"
    Goto UnLangDone

  ; German (0x0407)
  IntCmp $0 1031 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Deinstallation"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} wirklich deinstallieren?"
    StrCpy $LangRunning "${PRODUCT_NAME} wird ausgefuhrt. Bitte beenden Sie es zuerst!"
    StrCpy $LangTip "Hinweis"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Abbrechen"
    Goto UnLangDone

  ; French (0x040C)
  IntCmp $0 1036 0 +8 +8
    StrCpy $LangUninstallTitle "Desinstallation de ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Voulez-vous vraiment desinstaller ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} est en cours d'execution. Veuillez le fermer!"
    StrCpy $LangTip "Information"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuler"
    Goto UnLangDone

  ; Spanish (0x0C0A)
  IntCmp $0 3082 0 +8 +8
    StrCpy $LangUninstallTitle "Desinstalacion de ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Desea desinstalar ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} esta en ejecucion. Cierrelo primero!"
    StrCpy $LangTip "Aviso"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto UnLangDone

  ; Portuguese Brazil (0x0416)
  IntCmp $0 1046 0 +8 +8
    StrCpy $LangUninstallTitle "Desinstalacao do ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Deseja desinstalar o ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} esta em execucao. Feche-o primeiro!"
    StrCpy $LangTip "Aviso"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto UnLangDone

  ; Portuguese Portugal (0x0816)
  IntCmp $0 2070 0 +8 +8
    StrCpy $LangUninstallTitle "Desinstalacao do ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Deseja desinstalar o ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} esta em execucao. Feche-o primeiro!"
    StrCpy $LangTip "Aviso"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancelar"
    Goto UnLangDone

  ; Russian (0x0419)
  IntCmp $0 1049 0 +8 +8
    StrCpy $LangUninstallTitle "Udalenie ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Udalit ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} zapushen. Zakroyte ego!"
    StrCpy $LangTip "Uvedomlenie"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Otmena"
    Goto UnLangDone

  ; Italian (0x0410)
  IntCmp $0 1040 0 +8 +8
    StrCpy $LangUninstallTitle "Disinstallazione di ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Disinstallare ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} e in esecuzione. Chiuderlo prima!"
    StrCpy $LangTip "Avviso"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annulla"
    Goto UnLangDone

  ; Dutch (0x0413)
  IntCmp $0 1043 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Verwijderen"
    StrCpy $LangUninstallConfirm "${PRODUCT_NAME} verwijderen?"
    StrCpy $LangRunning "${PRODUCT_NAME} is actief. Sluit het eerst!"
    StrCpy $LangTip "Melding"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuleren"
    Goto UnLangDone

  ; Polish (0x0415)
  IntCmp $0 1045 0 +8 +8
    StrCpy $LangUninstallTitle "Deinstalacja ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Czy na pewno chcesz odinstalowac ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} jest uruchomiony. Zamknij go najpierw!"
    StrCpy $LangTip "Informacja"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Anuluj"
    Goto UnLangDone

  ; Czech (0x0405)
  IntCmp $0 1029 0 +8 +8
    StrCpy $LangUninstallTitle "Odinstalace ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Opravdu chcete odinstalovat ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} je spusten. Nejprve ho ukoncete!"
    StrCpy $LangTip "Upozorneni"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Zrusit"
    Goto UnLangDone

  ; Slovak (0x041B)
  IntCmp $0 1051 0 +8 +8
    StrCpy $LangUninstallTitle "Odinstalovanie ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Naozaj chcete odinstalovanie ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} je spusteny. Najprv ho ukoncite!"
    StrCpy $LangTip "Upozornenie"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Zrusit"
    Goto UnLangDone

  ; Hungarian (0x040E)
  IntCmp $0 1038 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Eltavolitas"
    StrCpy $LangUninstallConfirm "Biztosan el akarja tavolitani a ${PRODUCT_NAME} programot?"
    StrCpy $LangRunning "${PRODUCT_NAME} fut. Kerem, zarje be eloszor!"
    StrCpy $LangTip "Ertesites"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Megse"
    Goto UnLangDone

  ; Romanian (0x0418)
  IntCmp $0 1048 0 +8 +8
    StrCpy $LangUninstallTitle "Dezinstalare ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Sigur doriti sa dezinstalati ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} ruleaza. Inchideti-l mai intai!"
    StrCpy $LangTip "Notificare"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Anuleaza"
    Goto UnLangDone

  ; Swedish (0x041D)
  IntCmp $0 1053 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Avinstallation"
    StrCpy $LangUninstallConfirm "Vill du avinstallera ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} kors. Stang det forst!"
    StrCpy $LangTip "Meddelande"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Avbryt"
    Goto UnLangDone

  ; Danish (0x0406)
  IntCmp $0 1030 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Afinstallation"
    StrCpy $LangUninstallConfirm "Vil du afinstallere ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} korer. Luk det forst!"
    StrCpy $LangTip "Besked"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Annuller"
    Goto UnLangDone

  ; Norwegian Bokmal (0x0414)
  IntCmp $0 1044 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Avinstallasjon"
    StrCpy $LangUninstallConfirm "Vil du avinstallere ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} kjorer. Lukk det forst!"
    StrCpy $LangTip "Melding"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Avbryt"
    Goto UnLangDone

  ; Croatian (0x041A)
  IntCmp $0 1050 0 +8 +8
    StrCpy $LangUninstallTitle "Deinstalacija ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Zelite li deinstalirati ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} je pokrenut. Zatvorite ga prvo!"
    StrCpy $LangTip "Obavijest"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Odustani"
    Goto UnLangDone

  ; Greek (0x0408)
  IntCmp $0 1032 0 +8 +8
    StrCpy $LangUninstallTitle "Apegkatastasi ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Thelete na apegkatastisete to ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} ekteleite. Kleiste to prota!"
    StrCpy $LangTip "Eidopoiisi"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Akyrosi"
    Goto UnLangDone

  ; Catalan (0x0403)
  IntCmp $0 1027 0 +8 +8
    StrCpy $LangUninstallTitle "Desinstallacio de ${PRODUCT_NAME}"
    StrCpy $LangUninstallConfirm "Voleu desinstal-lar ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} s'esta executant. Tanqueu-lo primer!"
    StrCpy $LangTip "Avis"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancel-lar"
    Goto UnLangDone

  ; Irish (0x083C)
  IntCmp $0 2108 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
    StrCpy $LangUninstallConfirm "An bhfuil tu cinnte gur mian leat ${PRODUCT_NAME} a dhisuiteail?"
    StrCpy $LangRunning "${PRODUCT_NAME} ag rith. Dun e ar dtus!"
    StrCpy $LangTip "Fogra"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cealaigh"
    Goto UnLangDone

  ; Malayalam (0x044C) - falls back to English
  IntCmp $0 1100 0 +8 +8
    StrCpy $LangUninstallTitle "${PRODUCT_NAME} Uninstall"
    StrCpy $LangUninstallConfirm "Are you sure you want to uninstall ${PRODUCT_NAME}?"
    StrCpy $LangRunning "${PRODUCT_NAME} is running. Please close it first!"
    StrCpy $LangTip "Notice"
    StrCpy $LangConfirm "OK"
    StrCpy $LangCancel "Cancel"
    Goto UnLangDone

UnLangDone:
FunctionEnd
