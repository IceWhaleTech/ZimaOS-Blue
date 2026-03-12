package i18n

// This file registers translations for all supported languages beyond en-US and zh-CN.
// Only IM-facing messages (errors + media generation) are translated here.
// UI Review and iMessage keys fall back to en-US for non-translated languages.

func init() {
	// --- en-GB (English, UK) ---
	// Shares en-US translations (fallback handles it), only override where needed.

	// --- zh-TW (Traditional Chinese) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "抱歉，處理您的訊息時發生錯誤：%v",
		MsgChannelNotConnected:      "頻道未連線，請稍後重試。",
		MsgTimeout:                  "請求逾時，請重試。",
		MsgRateLimited:              "請求過於頻繁，請稍等片刻後重試。",
		MsgServiceUnavailable:       "服務暫時不可用，請稍後重試。",
		MsgInvalidRequest:           "無效的請求，請檢查您的輸入後重試。",
		MsgUnauthorized:             "您沒有權限執行此操作。",
		MsgInternalError:            "發生內部錯誤，請稍後重試。",
		MsgNoProviderAvailable:      "沒有可用的AI服務提供商，請檢查設定或聯繫管理員。",
		MsgProvidersInCooldown:      "暫時沒有可用的AI服務提供商（有 %d 個提供商正在冷卻中），請稍後重試。",
		MsgPathEscapesWorkspaceRoot: "路徑超出工作區根目錄",
		MsgMediaGenerating:          "🎨 正在生成媒體，完成後會傳送給你。",
		MsgMediaGenFailed:           "❌ 媒體生成失敗：%s",
		MsgMediaGenCancelled:        "🚫 媒體生成已取消。",
		MsgMediaGenNoOutput:         "✅ 生成完成，但沒有回傳結果。",
		MsgMediaImageGenerated:      "✅ 圖片已生成（%s）",
		MsgMediaVideoGenerated:      "✅ 影片已生成（%s）",
		MsgMediaGenerated:           "✅ 媒體已生成（%s）",
		MsgMediaGenTimeout:          "⏰ 媒體生成逾時，請重試。",
	} {
		AddTranslation(LangZhTW, k, v)
	}

	// --- ja-JP (Japanese) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "申し訳ありません。メッセージの処理中にエラーが発生しました：%v",
		MsgChannelNotConnected:      "チャンネルが接続されていません。後でもう一度お試しください。",
		MsgTimeout:                  "リクエストがタイムアウトしました。もう一度お試しください。",
		MsgRateLimited:              "リクエストが多すぎます。しばらくしてからもう一度お試しください。",
		MsgServiceUnavailable:       "サービスは一時的に利用できません。後でもう一度お試しください。",
		MsgInvalidRequest:           "無効なリクエストです。入力を確認してもう一度お試しください。",
		MsgUnauthorized:             "この操作を実行する権限がありません。",
		MsgInternalError:            "内部エラーが発生しました。後でもう一度お試しください。",
		MsgNoProviderAvailable:      "利用可能なAIサービスプロバイダーがありません。設定を確認するか、管理者にお問い合わせください。",
		MsgProvidersInCooldown:      "現在利用可能なAIサービスプロバイダーがありません（%d 件がクールダウン中）。後でもう一度お試しください。",
		MsgPathEscapesWorkspaceRoot: "パスがワークスペースのルートを超えています",
		MsgMediaGenerating:          "🎨 メディアを生成中です。完了したらお送りします。",
		MsgMediaGenFailed:           "❌ メディア生成に失敗しました：%s",
		MsgMediaGenCancelled:        "🚫 メディア生成がキャンセルされました。",
		MsgMediaGenNoOutput:         "✅ 生成は完了しましたが、出力がありませんでした。",
		MsgMediaImageGenerated:      "✅ 画像が生成されました（%s）",
		MsgMediaVideoGenerated:      "✅ 動画が生成されました（%s）",
		MsgMediaGenerated:           "✅ メディアが生成されました（%s）",
		MsgMediaGenTimeout:          "⏰ メディア生成がタイムアウトしました。もう一度お試しください。",
	} {
		AddTranslation(LangJaJP, k, v)
	}

	// --- ko-KR (Korean) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "죄송합니다. 메시지 처리 중 오류가 발생했습니다: %v",
		MsgChannelNotConnected:      "채널이 연결되지 않았습니다. 나중에 다시 시도해 주세요.",
		MsgTimeout:                  "요청 시간이 초과되었습니다. 다시 시도해 주세요.",
		MsgRateLimited:              "요청이 너무 많습니다. 잠시 후 다시 시도해 주세요.",
		MsgServiceUnavailable:       "서비스를 일시적으로 사용할 수 없습니다. 나중에 다시 시도해 주세요.",
		MsgInvalidRequest:           "잘못된 요청입니다. 입력을 확인하고 다시 시도해 주세요.",
		MsgUnauthorized:             "이 작업을 수행할 권한이 없습니다.",
		MsgInternalError:            "내부 오류가 발생했습니다. 나중에 다시 시도해 주세요.",
		MsgNoProviderAvailable:      "사용 가능한 AI 서비스 제공자가 없습니다. 설정을 확인하거나 관리자에게 문의하세요.",
		MsgProvidersInCooldown:      "현재 사용 가능한 AI 서비스 제공자가 없습니다 (%d개가 쿨다운 중). 나중에 다시 시도해 주세요.",
		MsgPathEscapesWorkspaceRoot: "경로가 워크스페이스 루트를 벗어났습니다",
		MsgMediaGenerating:          "🎨 미디어를 생성 중입니다. 완료되면 보내드리겠습니다.",
		MsgMediaGenFailed:           "❌ 미디어 생성 실패: %s",
		MsgMediaGenCancelled:        "🚫 미디어 생성이 취소되었습니다.",
		MsgMediaGenNoOutput:         "✅ 생성이 완료되었지만 출력이 없습니다.",
		MsgMediaImageGenerated:      "✅ 이미지가 생성되었습니다 (%s)",
		MsgMediaVideoGenerated:      "✅ 동영상이 생성되었습니다 (%s)",
		MsgMediaGenerated:           "✅ 미디어가 생성되었습니다 (%s)",
		MsgMediaGenTimeout:          "⏰ 미디어 생성 시간이 초과되었습니다. 다시 시도해 주세요.",
	} {
		AddTranslation(LangKoKR, k, v)
	}

	// --- de-DE (German) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Entschuldigung, bei der Verarbeitung Ihrer Nachricht ist ein Fehler aufgetreten: %v",
		MsgChannelNotConnected:      "Der Kanal ist nicht verbunden. Bitte versuchen Sie es später erneut.",
		MsgTimeout:                  "Die Anfrage ist abgelaufen. Bitte versuchen Sie es erneut.",
		MsgRateLimited:              "Zu viele Anfragen. Bitte warten Sie einen Moment und versuchen Sie es erneut.",
		MsgServiceUnavailable:       "Der Dienst ist vorübergehend nicht verfügbar. Bitte versuchen Sie es später erneut.",
		MsgInvalidRequest:           "Ungültige Anfrage. Bitte überprüfen Sie Ihre Eingabe und versuchen Sie es erneut.",
		MsgUnauthorized:             "Sie sind nicht berechtigt, diese Aktion auszuführen.",
		MsgInternalError:            "Ein interner Fehler ist aufgetreten. Bitte versuchen Sie es später erneut.",
		MsgNoProviderAvailable:      "Kein KI-Dienstanbieter verfügbar. Bitte überprüfen Sie die Konfiguration oder kontaktieren Sie den Administrator.",
		MsgProvidersInCooldown:      "Derzeit ist kein KI-Dienstanbieter verfügbar (%d Anbieter befinden sich in der Abkühlphase). Bitte versuchen Sie es später erneut.",
		MsgPathEscapesWorkspaceRoot: "Pfad liegt außerhalb des Workspace-Stammverzeichnisses",
		MsgMediaGenerating:          "🎨 Medien werden generiert... Ich sende das Ergebnis, wenn es fertig ist.",
		MsgMediaGenFailed:           "❌ Mediengenerierung fehlgeschlagen: %s",
		MsgMediaGenCancelled:        "🚫 Mediengenerierung wurde abgebrochen.",
		MsgMediaGenNoOutput:         "✅ Generierung abgeschlossen, aber keine Ausgabe erhalten.",
		MsgMediaImageGenerated:      "✅ Bild generiert (%s)",
		MsgMediaVideoGenerated:      "✅ Video generiert (%s)",
		MsgMediaGenerated:           "✅ Medien generiert (%s)",
		MsgMediaGenTimeout:          "⏰ Mediengenerierung abgelaufen. Bitte versuchen Sie es erneut.",
	} {
		AddTranslation(LangDeDE, k, v)
	}

	// --- fr-FR (French) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Désolé, une erreur s'est produite lors du traitement de votre message : %v",
		MsgChannelNotConnected:      "Le canal n'est pas connecté. Veuillez réessayer plus tard.",
		MsgTimeout:                  "La requête a expiré. Veuillez réessayer.",
		MsgRateLimited:              "Trop de requêtes. Veuillez patienter un moment et réessayer.",
		MsgServiceUnavailable:       "Le service est temporairement indisponible. Veuillez réessayer plus tard.",
		MsgInvalidRequest:           "Requête invalide. Veuillez vérifier votre saisie et réessayer.",
		MsgUnauthorized:             "Vous n'êtes pas autorisé à effectuer cette action.",
		MsgInternalError:            "Une erreur interne s'est produite. Veuillez réessayer plus tard.",
		MsgNoProviderAvailable:      "Aucun fournisseur de service IA disponible. Veuillez vérifier la configuration ou contacter l'administrateur.",
		MsgProvidersInCooldown:      "Aucun fournisseur de service IA n'est actuellement disponible (%d fournisseurs en refroidissement). Veuillez réessayer plus tard.",
		MsgPathEscapesWorkspaceRoot: "Le chemin sort de la racine de l'espace de travail",
		MsgMediaGenerating:          "🎨 Génération de média en cours... Je vous enverrai le résultat quand ce sera prêt.",
		MsgMediaGenFailed:           "❌ Échec de la génération de média : %s",
		MsgMediaGenCancelled:        "🚫 La génération de média a été annulée.",
		MsgMediaGenNoOutput:         "✅ Génération terminée, mais aucun résultat n'a été retourné.",
		MsgMediaImageGenerated:      "✅ Image générée (%s)",
		MsgMediaVideoGenerated:      "✅ Vidéo générée (%s)",
		MsgMediaGenerated:           "✅ Média généré (%s)",
		MsgMediaGenTimeout:          "⏰ La génération de média a expiré. Veuillez réessayer.",
	} {
		AddTranslation(LangFrFR, k, v)
	}

	// --- es-ES (Spanish) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Lo siento, se produjo un error al procesar su mensaje: %v",
		MsgChannelNotConnected:      "El canal no está conectado. Por favor, inténtelo más tarde.",
		MsgTimeout:                  "La solicitud ha expirado. Por favor, inténtelo de nuevo.",
		MsgRateLimited:              "Demasiadas solicitudes. Por favor, espere un momento e inténtelo de nuevo.",
		MsgServiceUnavailable:       "El servicio no está disponible temporalmente. Por favor, inténtelo más tarde.",
		MsgInvalidRequest:           "Solicitud no válida. Por favor, verifique su entrada e inténtelo de nuevo.",
		MsgUnauthorized:             "No tiene autorización para realizar esta acción.",
		MsgInternalError:            "Se produjo un error interno. Por favor, inténtelo más tarde.",
		MsgNoProviderAvailable:      "No hay proveedor de servicio de IA disponible. Por favor, verifique la configuración o contacte al administrador.",
		MsgProvidersInCooldown:      "No hay proveedor de servicio de IA disponible actualmente (%d proveedores en enfriamiento). Por favor, inténtelo más tarde.",
		MsgPathEscapesWorkspaceRoot: "La ruta se sale de la raíz del espacio de trabajo",
		MsgMediaGenerating:          "🎨 Generando contenido multimedia... Te enviaré el resultado cuando esté listo.",
		MsgMediaGenFailed:           "❌ Error en la generación de multimedia: %s",
		MsgMediaGenCancelled:        "🚫 La generación de multimedia fue cancelada.",
		MsgMediaGenNoOutput:         "✅ Generación completada, pero no se obtuvo resultado.",
		MsgMediaImageGenerated:      "✅ Imagen generada (%s)",
		MsgMediaVideoGenerated:      "✅ Vídeo generado (%s)",
		MsgMediaGenerated:           "✅ Multimedia generado (%s)",
		MsgMediaGenTimeout:          "⏰ La generación de multimedia ha expirado. Por favor, inténtelo de nuevo.",
	} {
		AddTranslation(LangEsES, k, v)
	}

	// --- it-IT (Italian) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Spiacente, si è verificato un errore durante l'elaborazione del messaggio: %v",
		MsgChannelNotConnected:      "Il canale non è connesso. Riprova più tardi.",
		MsgTimeout:                  "La richiesta è scaduta. Riprova.",
		MsgRateLimited:              "Troppe richieste. Attendi un momento e riprova.",
		MsgServiceUnavailable:       "Il servizio è temporaneamente non disponibile. Riprova più tardi.",
		MsgInvalidRequest:           "Richiesta non valida. Controlla il tuo input e riprova.",
		MsgUnauthorized:             "Non sei autorizzato a eseguire questa azione.",
		MsgInternalError:            "Si è verificato un errore interno. Riprova più tardi.",
		MsgNoProviderAvailable:      "Nessun fornitore di servizi IA disponibile. Controlla la configurazione o contatta l'amministratore.",
		MsgProvidersInCooldown:      "Nessun fornitore di servizi IA attualmente disponibile (%d fornitori in raffreddamento). Riprova più tardi.",
		MsgPathEscapesWorkspaceRoot: "Il percorso esce dalla radice dell'area di lavoro",
		MsgMediaGenerating:          "🎨 Generazione media in corso... Ti invierò il risultato quando sarà pronto.",
		MsgMediaGenFailed:           "❌ Generazione media fallita: %s",
		MsgMediaGenCancelled:        "🚫 La generazione media è stata annullata.",
		MsgMediaGenNoOutput:         "✅ Generazione completata, ma nessun risultato restituito.",
		MsgMediaImageGenerated:      "✅ Immagine generata (%s)",
		MsgMediaVideoGenerated:      "✅ Video generato (%s)",
		MsgMediaGenerated:           "✅ Media generato (%s)",
		MsgMediaGenTimeout:          "⏰ Generazione media scaduta. Riprova.",
	} {
		AddTranslation(LangItIT, k, v)
	}

	// --- pt-BR (Portuguese, Brazil) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Desculpe, ocorreu um erro ao processar sua mensagem: %v",
		MsgChannelNotConnected:      "O canal não está conectado. Tente novamente mais tarde.",
		MsgTimeout:                  "A solicitação expirou. Tente novamente.",
		MsgRateLimited:              "Muitas solicitações. Aguarde um momento e tente novamente.",
		MsgServiceUnavailable:       "O serviço está temporariamente indisponível. Tente novamente mais tarde.",
		MsgInvalidRequest:           "Solicitação inválida. Verifique sua entrada e tente novamente.",
		MsgUnauthorized:             "Você não tem autorização para realizar esta ação.",
		MsgInternalError:            "Ocorreu um erro interno. Tente novamente mais tarde.",
		MsgNoProviderAvailable:      "Nenhum provedor de serviço de IA disponível. Verifique a configuração ou entre em contato com o administrador.",
		MsgProvidersInCooldown:      "Nenhum provedor de serviço de IA disponível no momento (%d provedores em resfriamento). Tente novamente mais tarde.",
		MsgPathEscapesWorkspaceRoot: "O caminho sai da raiz do espaço de trabalho",
		MsgMediaGenerating:          "🎨 Gerando mídia... Enviarei o resultado quando estiver pronto.",
		MsgMediaGenFailed:           "❌ Falha na geração de mídia: %s",
		MsgMediaGenCancelled:        "🚫 A geração de mídia foi cancelada.",
		MsgMediaGenNoOutput:         "✅ Geração concluída, mas nenhum resultado foi retornado.",
		MsgMediaImageGenerated:      "✅ Imagem gerada (%s)",
		MsgMediaVideoGenerated:      "✅ Vídeo gerado (%s)",
		MsgMediaGenerated:           "✅ Mídia gerada (%s)",
		MsgMediaGenTimeout:          "⏰ A geração de mídia expirou. Tente novamente.",
	} {
		AddTranslation(LangPtBR, k, v)
	}

	// --- pt-PT (Portuguese, Portugal) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Lamentamos, ocorreu um erro ao processar a sua mensagem: %v",
		MsgChannelNotConnected:      "O canal não está ligado. Tente novamente mais tarde.",
		MsgTimeout:                  "O pedido expirou. Tente novamente.",
		MsgRateLimited:              "Demasiados pedidos. Aguarde um momento e tente novamente.",
		MsgServiceUnavailable:       "O serviço está temporariamente indisponível. Tente novamente mais tarde.",
		MsgInvalidRequest:           "Pedido inválido. Verifique a sua entrada e tente novamente.",
		MsgUnauthorized:             "Não tem autorização para realizar esta ação.",
		MsgInternalError:            "Ocorreu um erro interno. Tente novamente mais tarde.",
		MsgNoProviderAvailable:      "Nenhum fornecedor de serviço de IA disponível. Verifique a configuração ou contacte o administrador.",
		MsgProvidersInCooldown:      "Nenhum fornecedor de serviço de IA disponível de momento (%d fornecedores em arrefecimento). Tente novamente mais tarde.",
		MsgPathEscapesWorkspaceRoot: "O caminho sai da raiz da área de trabalho",
		MsgMediaGenerating:          "🎨 A gerar média... Enviarei o resultado quando estiver pronto.",
		MsgMediaGenFailed:           "❌ Falha na geração de média: %s",
		MsgMediaGenCancelled:        "🚫 A geração de média foi cancelada.",
		MsgMediaGenNoOutput:         "✅ Geração concluída, mas nenhum resultado foi devolvido.",
		MsgMediaImageGenerated:      "✅ Imagem gerada (%s)",
		MsgMediaVideoGenerated:      "✅ Vídeo gerado (%s)",
		MsgMediaGenerated:           "✅ Média gerada (%s)",
		MsgMediaGenTimeout:          "⏰ A geração de média expirou. Tente novamente.",
	} {
		AddTranslation(LangPtPT, k, v)
	}

	// --- ru-RU (Russian) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Извините, при обработке вашего сообщения произошла ошибка: %v",
		MsgChannelNotConnected:      "Канал не подключён. Попробуйте позже.",
		MsgTimeout:                  "Время запроса истекло. Попробуйте ещё раз.",
		MsgRateLimited:              "Слишком много запросов. Подождите немного и попробуйте снова.",
		MsgServiceUnavailable:       "Сервис временно недоступен. Попробуйте позже.",
		MsgInvalidRequest:           "Неверный запрос. Проверьте введённые данные и попробуйте снова.",
		MsgUnauthorized:             "У вас нет прав для выполнения этого действия.",
		MsgInternalError:            "Произошла внутренняя ошибка. Попробуйте позже.",
		MsgNoProviderAvailable:      "Нет доступного поставщика ИИ-услуг. Проверьте конфигурацию или обратитесь к администратору.",
		MsgProvidersInCooldown:      "В данный момент нет доступного поставщика ИИ-услуг (%d в режиме ожидания). Попробуйте позже.",
		MsgPathEscapesWorkspaceRoot: "Путь выходит за пределы корня рабочей области",
		MsgMediaGenerating:          "🎨 Генерация медиа... Отправлю результат, когда будет готово.",
		MsgMediaGenFailed:           "❌ Ошибка генерации медиа: %s",
		MsgMediaGenCancelled:        "🚫 Генерация медиа отменена.",
		MsgMediaGenNoOutput:         "✅ Генерация завершена, но результат не получен.",
		MsgMediaImageGenerated:      "✅ Изображение сгенерировано (%s)",
		MsgMediaVideoGenerated:      "✅ Видео сгенерировано (%s)",
		MsgMediaGenerated:           "✅ Медиа сгенерировано (%s)",
		MsgMediaGenTimeout:          "⏰ Время генерации медиа истекло. Попробуйте ещё раз.",
	} {
		AddTranslation(LangRuRU, k, v)
	}
}
