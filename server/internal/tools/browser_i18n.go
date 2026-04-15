package tools

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

func browserToolLanguage(lang string) i18n.Language {
	return i18n.ParseLanguage(strings.TrimSpace(lang))
}

func BrowserScreenshotMessage(lang i18n.Language, url, targetID string) string {
	switch {
	case strings.TrimSpace(url) != "":
		return i18n.T(lang, i18n.MsgBrowserScreenshotCapturedFor, strings.TrimSpace(url))
	case strings.TrimSpace(targetID) != "":
		return i18n.T(lang, i18n.MsgBrowserScreenshotCapturedForTab, strings.TrimSpace(targetID))
	default:
		return i18n.T(lang, i18n.MsgBrowserScreenshotCapturedForActiveTab)
	}
}

func BrowserScreenshotInteractiveUnavailableMessage(lang i18n.Language) string {
	return i18n.T(lang, i18n.MsgBrowserScreenshotCapturedInteractiveUnavailable)
}

var browserDirectionLabels = map[i18n.Language]map[string]string{
	i18n.LangZhCN: {"up": "上", "down": "下", "left": "左", "right": "右"},
	i18n.LangZhTW: {"up": "上", "down": "下", "left": "左", "right": "右"},
	i18n.LangJaJP: {"up": "上", "down": "下", "left": "左", "right": "右"},
	i18n.LangKoKR: {"up": "위", "down": "아래", "left": "왼쪽", "right": "오른쪽"},
	i18n.LangDeDE: {"up": "oben", "down": "unten", "left": "links", "right": "rechts"},
	i18n.LangFrFR: {"up": "haut", "down": "bas", "left": "gauche", "right": "droite"},
	i18n.LangEsES: {"up": "arriba", "down": "abajo", "left": "la izquierda", "right": "la derecha"},
	i18n.LangItIT: {"up": "l'alto", "down": "il basso", "left": "sinistra", "right": "destra"},
	i18n.LangPtBR: {"up": "cima", "down": "baixo", "left": "esquerda", "right": "direita"},
	i18n.LangPtPT: {"up": "cima", "down": "baixo", "left": "a esquerda", "right": "a direita"},
	i18n.LangRuRU: {"up": "вверх", "down": "вниз", "left": "влево", "right": "вправо"},
	i18n.LangPlPL: {"up": "górę", "down": "dół", "left": "lewo", "right": "prawo"},
	i18n.LangNlNL: {"up": "boven", "down": "beneden", "left": "links", "right": "rechts"},
	i18n.LangSvSE: {"up": "upp", "down": "ned", "left": "vänster", "right": "höger"},
	i18n.LangDaDK: {"up": "op", "down": "ned", "left": "til venstre", "right": "til højre"},
	i18n.LangNbNO: {"up": "opp", "down": "ned", "left": "til venstre", "right": "til høyre"},
	i18n.LangCsCZ: {"up": "nahoru", "down": "dolů", "left": "doleva", "right": "doprava"},
	i18n.LangSkSK: {"up": "nahor", "down": "nadol", "left": "doľava", "right": "doprava"},
	i18n.LangHuHU: {"up": "fel", "down": "le", "left": "balra", "right": "jobbra"},
	i18n.LangRoRO: {"up": "în sus", "down": "în jos", "left": "la stânga", "right": "la dreapta"},
	i18n.LangHrHR: {"up": "gore", "down": "dolje", "left": "lijevo", "right": "desno"},
	i18n.LangElGR: {"up": "πάνω", "down": "κάτω", "left": "αριστερά", "right": "δεξιά"},
	i18n.LangCaES: {"up": "amunt", "down": "avall", "left": "l'esquerra", "right": "la dreta"},
	i18n.LangGaIE: {"up": "suas", "down": "síos", "left": "ar chlé", "right": "ar dheis"},
	i18n.LangMlIN: {"up": "മുകളിൽ", "down": "താഴേക്ക്", "left": "ഇടത്തേക്ക്", "right": "വലത്തേക്ക്"},
}

func browserDirectionLabel(lang i18n.Language, direction string) string {
	normalized := strings.ToLower(strings.TrimSpace(direction))
	if labels, ok := browserDirectionLabels[lang]; ok {
		if localized, ok := labels[normalized]; ok {
			return localized
		}
	}
	return normalized
}

func BrowserActionPerformedMessage(lang i18n.Language, actType string, ref int) string {
	return i18n.T(lang, i18n.MsgBrowserActionPerformedOnRef, strings.TrimSpace(actType), ref)
}

func BrowserPageScrolledMessage(lang i18n.Language, direction string) string {
	return i18n.T(lang, i18n.MsgBrowserScrolledPage, browserDirectionLabel(lang, direction))
}

func BrowserOpenTabsMessage(lang i18n.Language, count int) string {
	return i18n.T(lang, i18n.MsgBrowserOpenTabs, count)
}

func BrowserTabClosedMessage(lang i18n.Language, targetID string) string {
	return i18n.T(lang, i18n.MsgBrowserTabClosed, strings.TrimSpace(targetID))
}

func BrowserRecipesAvailableMessage(lang i18n.Language, count int) string {
	return i18n.T(lang, i18n.MsgBrowserRecipesAvailable, count)
}

func BrowserLargeDOMNoteMessage(lang i18n.Language, count int) string {
	return i18n.T(lang, i18n.MsgBrowserLargeDOMNote, count)
}

func BrowserPageMessage(lang i18n.Language, title, url, tree string, count int) string {
	if count >= 0 {
		return i18n.T(lang, i18n.MsgBrowserPageWithInteractiveCount, title, url, count, tree)
	}
	return i18n.T(lang, i18n.MsgBrowserPage, title, url, tree)
}

func BrowserPageWithScreenshotInteractiveMessage(lang i18n.Language, title, url, tree string, count int) string {
	return i18n.T(lang, i18n.MsgBrowserPageWithScreenshotInteractiveCount, title, url, count, tree)
}

func BrowserReadableContentMessage(lang i18n.Language, title, url, content string) string {
	return i18n.T(lang, i18n.MsgBrowserPageMainContent, title, url, content)
}

func BrowserReadableContentWithTreeMessage(
	lang i18n.Language,
	title, url, content, strategy, tree string,
) string {
	sectionKey := i18n.MsgBrowserPageStructureSection
	switch strings.TrimSpace(strategy) {
	case "interactive", "screenshot+interactive":
		sectionKey = i18n.MsgBrowserInteractiveElementsSection
	}
	sectionLabel := i18n.T(lang, sectionKey)
	return i18n.T(lang, i18n.MsgBrowserPageMainContentWithSection, title, url, content, sectionLabel, tree)
}
