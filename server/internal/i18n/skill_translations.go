package i18n

func registerDeclarativeSkillTranslations() {
	for lang, value := range map[Language]string{
		LangEnUS: `Skill %[1]q is declarative — handled by LLM`,
		LangEnGB: `Skill %[1]q is declarative — handled by LLM`,
		LangZhCN: `技能 %[1]q 是声明式的，由 LLM 直接处理`,
		LangZhTW: `技能 %[1]q 是宣告式的，由 LLM 直接處理`,
		LangJaJP: `スキル %[1]q は宣言型で、LLM が直接処理します`,
		LangKoKR: `스킬 %[1]q 은 선언형이며 LLM이 직접 처리합니다`,
		LangDeDE: `Skill %[1]q ist deklarativ und wird direkt vom LLM verarbeitet`,
		LangFrFR: `La compétence %[1]q est déclarative et est traitée directement par le LLM`,
		LangEsES: `La habilidad %[1]q es declarativa y la gestiona directamente el LLM`,
		LangItIT: `La skill %[1]q è dichiarativa ed è gestita direttamente dall'LLM`,
		LangPtBR: `A skill %[1]q é declarativa e é tratada diretamente pelo LLM`,
		LangPtPT: `A skill %[1]q é declarativa e é tratada diretamente pelo LLM`,
		LangRuRU: `Навык %[1]q является декларативным и обрабатывается самим LLM`,
		LangPlPL: `Umiejętność %[1]q jest deklaratywna i jest obsługiwana bezpośrednio przez LLM`,
		LangNlNL: `Skill %[1]q is declaratief en wordt rechtstreeks door de LLM afgehandeld`,
		LangSvSE: `Färdigheten %[1]q är deklarativ och hanteras direkt av LLM`,
		LangDaDK: `Færdigheden %[1]q er deklarativ og håndteres direkte af LLM`,
		LangNbNO: `Ferdigheten %[1]q er deklarativ og håndteres direkte av LLM`,
		LangCsCZ: `Dovednost %[1]q je deklarativní a zpracovává ji přímo LLM`,
		LangSkSK: `Zručnosť %[1]q je deklaratívna a spracúva ju priamo LLM`,
		LangHuHU: `A(z) %[1]q készség deklaratív, és közvetlenül az LLM kezeli`,
		LangRoRO: `Skillul %[1]q este declarativ și este gestionat direct de LLM`,
		LangHrHR: `Vještina %[1]q je deklarativna i njome izravno upravlja LLM`,
		LangElGR: `Η δεξιότητα %[1]q είναι δηλωτική και διαχειρίζεται απευθείας από το LLM`,
		LangCaES: `La skill %[1]q és declarativa i la gestiona directament l'LLM`,
		LangGaIE: `Tá an scil %[1]q dearbhaitheach agus láimhseálann an LLM go díreach í`,
		LangMlIN: `സ്കിൽ %[1]q പ്രഖ്യാപനാത്മകമാണ്, ഇത് നേരിട്ട് LLM കൈകാര്യം ചെയ്യുന്നു`,
	} {
		AddTranslation(lang, MsgSkillDeclarativeHandledByLLM, value)
	}
}
