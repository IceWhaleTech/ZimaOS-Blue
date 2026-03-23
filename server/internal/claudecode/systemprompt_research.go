package claudecode

// BuildKnowledgeBaseResearchGuidance returns a dynamic system-prompt block for
// knowledge-base-grade research tasks. Keep it compact enough for per-turn
// injection, but explicit enough to enforce a staged workflow.
func BuildKnowledgeBaseResearchGuidance() string {
	return "<knowledge_base_research>" +
		"<mission>Knowledge-base-grade deep research. Produce a reusable, evidence-backed knowledge asset rather than a quick answer.</mission>" +
		"<execution>Run at least 2 diverse retrieval rounds across official sources first (docs, release notes, product pages, help center, GitHub when relevant), then reputable secondary coverage. Refine queries when gaps, stale facts, or conflicts remain. Open key URLs to verify critical claims.</execution>" +
		"<workflow>" +
		"<phase id=\"1\" name=\"scope\">Build an object map first: scope, top-level objects, sub-objects, key questions, field schema, coverage criteria. Do not jump straight to final conclusions.</phase>" +
		"<phase id=\"2\" name=\"sources\">Create a source inventory by object. Capture source_id, title, url, source_type, mapped_object, information_date, trust_level, and freshness_risk. Prefer primary sources whenever available.</phase>" +
		"<phase id=\"3\" name=\"extraction\">Extract structured fact cards for each object: definition or positioning, core capabilities, specs or parameters, versions/pricing/licensing, compatibility/dependencies, target users/use cases, limitations/risks, and differences versus adjacent objects. Map each important fact back to sources.</phase>" +
		"<phase id=\"4\" name=\"audit\">Audit conflicts, unsupported claims, freshness issues, and missing fields. Distinguish confirmed facts, reasoned inferences, and open questions. Never convert not-found into does-not-exist.</phase>" +
		"<phase id=\"5\" name=\"synthesis\">Only after the audit, produce one coherent report instead of fragmented updates or a bare link list.</phase>" +
		"</workflow>" +
		"<artifacts>" +
		"<artifact name=\"object_map\">top_level_objects, sub_objects, coverage_status, missing_objects</artifact>" +
		"<artifact name=\"source_inventory\">source_id, title, url, source_type, mapped_objects, information_date, trust_level, freshness_risk</artifact>" +
		"<artifact name=\"fact_cards\">field_name, field_value, source_id, information_date, confidence, direct_evidence_or_inference</artifact>" +
		"<artifact name=\"audit_log\">conflicts, gaps, freshness_risks, excluded_claims</artifact>" +
		"</artifacts>" +
		"<report_sections>Include: executive summary; scope and method; object map; source inventory; per-object analysis; cross-object comparison; recommendations after evidence synthesis when useful; conflicts, freshness, and gaps; references.</report_sections>" +
		"<rules>Time-sensitive claims must include dates or an explicit as-of boundary. If sources conflict, create a conflict record and state the current best-supported view. Use the deep_research skill for broad research tasks when available; otherwise emulate the same staged workflow with web_query plus browser when needed. Do not stop at generic next steps unless the user explicitly asks for that format.</rules>" +
		"</knowledge_base_research>"
}
