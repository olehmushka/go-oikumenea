// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

// Hand-curated translations for the reference catalogs with no multilingual upstream (D-Pinax + D-i18n,
// M45): colors, religions and ranks in ukr/eng/spa/por. CLDR covers countries/scripts/languages; these
// three are curated. Coverage is the well-known set — uncurated codes stay English-only (no row emitted).

// nm is a ukr/eng/spa/por name tuple.
type nm struct{ ukr, eng, spa, por string }

func (n nm) names() map[string]string {
	m := map[string]string{}
	for k, v := range map[string]string{"ukr": n.ukr, "eng": n.eng, "spa": n.spa, "por": n.por} {
		if v != "" {
			m[k] = v
		}
	}
	return m
}

// ---- colors (per (domain, code); resolved to the color RID by the handler) ----

// colorWords maps a palette code to its four-locale names.
var colorWords = map[string]nm{
	"blue":    {"Синій", "Blue", "Azul", "Azul"},
	"yellow":  {"Жовтий", "Yellow", "Amarillo", "Amarelo"},
	"red":     {"Червоний", "Red", "Rojo", "Vermelho"},
	"gold":    {"Золотий", "Gold", "Dorado", "Dourado"},
	"silver":  {"Срібний", "Silver", "Plateado", "Prateado"},
	"white":   {"Білий", "White", "Blanco", "Branco"},
	"black":   {"Чорний", "Black", "Negro", "Preto"},
	"green":   {"Зелений", "Green", "Verde", "Verde"},
	"brown":   {"Коричневий", "Brown", "Marrón", "Marrom"},
	"hazel":   {"Горіховий", "Hazel", "Avellana", "Castanho-claro"},
	"grey":    {"Сірий", "Grey", "Gris", "Cinza"},
	"blonde":  {"Білявий", "Blonde", "Rubio", "Louro"},
	"neutral": {"Нейтральний", "Neutral", "Neutral", "Neutro"},
}

// colorTargets are the (domain, code) palette entries to translate (pinax `colors` preset + the
// migration eye/hair/vehicle palettes). Unresolved pairs are skipped at seed time.
var colorTargets = [][2]string{
	{"country", "blue"}, {"country", "yellow"}, {"country", "red"},
	{"rank", "gold"}, {"rank", "silver"},
	{"religion", "gold"}, {"religion", "white"},
	{"ethnicity", "neutral"},
	{"eye", "brown"}, {"eye", "blue"}, {"eye", "green"}, {"eye", "hazel"}, {"eye", "grey"},
	{"hair", "black"}, {"hair", "brown"}, {"hair", "blonde"}, {"hair", "red"}, {"hair", "grey"},
	{"vehicle", "white"}, {"vehicle", "black"}, {"vehicle", "silver"}, {"vehicle", "green"},
}

func curatedColorTranslations() []map[string]any {
	var out []map[string]any
	for _, t := range colorTargets {
		if w, ok := colorWords[t[1]]; ok {
			out = append(out, translationRecord("color", t[0]+"/"+t[1], w.names()))
		}
	}
	return out
}

// ---- religions (by taxon code; roots + major branches) ----

var religionNames = map[string]nm{
	"christianity":   {"Християнство", "Christianity", "Cristianismo", "Cristianismo"},
	"islam":          {"Іслам", "Islam", "Islam", "Islão"},
	"judaism":        {"Юдаїзм", "Judaism", "Judaísmo", "Judaísmo"},
	"hinduism":       {"Індуїзм", "Hinduism", "Hinduismo", "Hinduísmo"},
	"buddhism":       {"Буддизм", "Buddhism", "Budismo", "Budismo"},
	"sikhism":        {"Сикхізм", "Sikhism", "Sijismo", "Siquismo"},
	"jainism":        {"Джайнізм", "Jainism", "Jainismo", "Jainismo"},
	"bahai":          {"Віра Багаї", "Bahá'í Faith", "Fe bahá'í", "Fé bahá'í"},
	"shinto":         {"Синтоїзм", "Shinto", "Sintoísmo", "Xintoísmo"},
	"taoism":         {"Даосизм", "Taoism", "Taoísmo", "Taoísmo"},
	"confucianism":   {"Конфуціанство", "Confucianism", "Confucianismo", "Confucionismo"},
	"zoroastrianism": {"Зороастризм", "Zoroastrianism", "Zoroastrismo", "Zoroastrismo"},
	"atheism":        {"Атеїзм", "Atheism", "Ateísmo", "Ateísmo"},
	"agnosticism":    {"Агностицизм", "Agnosticism", "Agnosticismo", "Agnosticismo"},
	"traditional":    {"Традиційні релігії", "Traditional religions", "Religiones tradicionales", "Religiões tradicionais"},
	"other":          {"Інше", "Other", "Otro", "Outro"},

	"catholicism":        {"Католицизм", "Catholicism", "Catolicismo", "Catolicismo"},
	"eastern_orthodoxy":  {"Православ'я", "Eastern Orthodoxy", "Ortodoxia oriental", "Ortodoxia oriental"},
	"oriental_orthodoxy": {"Давньосхідні церкви", "Oriental Orthodoxy", "Ortodoxia oriental antigua", "Ortodoxia oriental antiga"},
	"protestantism":      {"Протестантизм", "Protestantism", "Protestantismo", "Protestantismo"},
	"sunni":              {"Сунізм", "Sunni Islam", "Islam suní", "Islão sunita"},
	"shia":               {"Шиїзм", "Shia Islam", "Islam chií", "Islão xiita"},
	"sufism":             {"Суфізм", "Sufism", "Sufismo", "Sufismo"},
	"orthodox_judaism":   {"Ортодоксальний юдаїзм", "Orthodox Judaism", "Judaísmo ortodoxo", "Judaísmo ortodoxo"},
	"reform_judaism":     {"Реформістський юдаїзм", "Reform Judaism", "Judaísmo reformista", "Judaísmo reformista"},
	"vaishnavism":        {"Вайшнавізм", "Vaishnavism", "Vaisnavismo", "Vixnuísmo"},
	"shaivism":           {"Шайвізм", "Shaivism", "Shaivismo", "Xivaísmo"},
	"theravada":          {"Тхеравада", "Theravāda", "Theravāda", "Teravada"},
	"mahayana":           {"Махаяна", "Mahāyāna", "Mahāyāna", "Maaiana"},
	"vajrayana":          {"Ваджраяна", "Vajrayāna", "Vajrayāna", "Vajrayana"},
	"lutheranism":        {"Лютеранство", "Lutheranism", "Luteranismo", "Luteranismo"},
	"anglicanism":        {"Англіканство", "Anglicanism", "Anglicanismo", "Anglicanismo"},
	"methodism":          {"Методизм", "Methodism", "Metodismo", "Metodismo"},
	"baptist":            {"Баптизм", "Baptist", "Bautismo", "Batismo"},
	"pentecostalism":     {"П'ятдесятництво", "Pentecostalism", "Pentecostalismo", "Pentecostalismo"},
}

func curatedReligionTranslations() []map[string]any {
	out := make([]map[string]any, 0, len(religionNames))
	for code, n := range religionNames {
		out = append(out, translationRecord("religion_taxon", code, n.names()))
	}
	return out
}

// ---- ranks (spa/por only; keyed by default English/Ukrainian name) ----
// rankDict maps a rank/category/type default name to its [spa, por]. The ukr/eng slots are the default
// `name` column already, so only the two new locales are curated. Covers the standard military terms;
// uncurated names stay English/Ukrainian only.
var rankDict = map[string][2]string{
	// categories / branches
	"Army":              {"Ejército", "Exército"},
	"Ground Forces":     {"Fuerzas Terrestres", "Forças Terrestres"},
	"Navy":              {"Armada", "Marinha"},
	"Air Force":         {"Fuerza Aérea", "Força Aérea"},
	"Marine Corps":      {"Cuerpo de Marines", "Corpo de Fuzileiros"},
	"Space Force":       {"Fuerza Espacial", "Força Espacial"},
	"Coast Guard":       {"Guardia Costera", "Guarda Costeira"},
	"Сухопутні війська": {"Fuerzas Terrestres", "Forças Terrestres"},
	"Військово-Морські Сили": {"Armada", "Marinha"},
	// type tiers
	"Enlisted":         {"Tropa y marinería", "Praças"},
	"Warrant Officers": {"Suboficiales técnicos", "Oficiais técnicos"},
	"Officers":         {"Oficiales", "Oficiais"},
	// common rank names (US)
	"Private":              {"Soldado", "Soldado"},
	"Corporal":             {"Cabo", "Cabo"},
	"Sergeant":             {"Sargento", "Sargento"},
	"Staff Sergeant":       {"Sargento primero", "Segundo-sargento"},
	"Sergeant First Class": {"Sargento de primera clase", "Primeiro-sargento"},
	"Master Sergeant":      {"Sargento maestre", "Sargento-mor"},
	"First Sergeant":       {"Sargento primero", "Sargento-ajudante"},
	"Sergeant Major":       {"Sargento mayor", "Sargento-mor"},
	"Lieutenant":           {"Teniente", "Tenente"},
	"Second Lieutenant":    {"Subteniente", "Segundo-tenente"},
	"First Lieutenant":     {"Teniente", "Primeiro-tenente"},
	"Captain":              {"Capitán", "Capitão"},
	"Major":                {"Comandante", "Major"},
	"Lieutenant Colonel":   {"Teniente coronel", "Tenente-coronel"},
	"Colonel":              {"Coronel", "Coronel"},
	"Brigadier General":    {"General de brigada", "General de brigada"},
	"Major General":        {"General de división", "General de divisão"},
	"Lieutenant General":   {"Teniente general", "Tenente-general"},
	"General":              {"General", "General"},
	"Admiral":              {"Almirante", "Almirante"},
	"Vice Admiral":         {"Vicealmirante", "Vice-almirante"},
	"Rear Admiral":         {"Contralmirante", "Contra-almirante"},
	"Ensign":               {"Alférez de fragata", "Segundo-tenente"},
	"Seaman":               {"Marinero", "Marinheiro"},
	"Airman":               {"Aviador", "Aviador"},
	// common rank names (UA, by Ukrainian default)
	"Солдат":    {"Soldado", "Soldado"},
	"Сержант":   {"Sargento", "Sargento"},
	"Лейтенант": {"Teniente", "Tenente"},
	"Капітан":   {"Capitán", "Capitão"},
	"Майор":     {"Comandante", "Major"},
	"Полковник": {"Coronel", "Coronel"},
	"Генерал":   {"General", "General"},
	"Адмірал":   {"Almirante", "Almirante"},
	"Матрос":    {"Marinero", "Marinheiro"},
}
