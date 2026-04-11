import type { LocaleKey } from './locale-catalog'

export interface CompletionFollowupChatMessages {
  completionFollowupHeading: string
  completionFollowupNoFurtherActionNeeded: string
  completionFollowupExpandFullerReport: string
  completionFollowupVerifyKeyEvidence: string
  completionFollowupReorderTakeaways: string
  completionFollowupOptimizationIdeas: string
  completionFollowupInspectFailedSteps: string
  completionFollowupRerunValidation: string
  completionFollowupKeepFixing: string
  completionFollowupVerifyDeliverables: string
  completionFollowupRunRelevantTests: string
  completionFollowupOptimizeNextArea: string
  completionFollowupIfYoudLikePrefix: string
  completionFollowupIfYouWantPrefix: string
}

function buildCompletionFollowupBackfill(copy: CompletionFollowupChatMessages) {
  return {
    chat: copy,
  }
}

const completionFollowupCopies: Partial<Record<LocaleKey, CompletionFollowupChatMessages>> = {
  'ca-ES': {
    completionFollowupHeading: 'Si vols, també et puc ajudar amb:',
    completionFollowupNoFurtherActionNeeded: 'No cal cap altra acció.',
    completionFollowupExpandFullerReport: 'Si vols, puc ampliar això en un informe més complet.',
    completionFollowupVerifyKeyEvidence:
      "Si vols, puc ajudar a verificar les proves clau de les targetes d'eines de dalt.",
    completionFollowupReorderTakeaways:
      'Si vols, digues-me la teva prioritat principal (per exemple, rendiment/cost/risc) i reordenaré les conclusions per a tu.',
    completionFollowupOptimizationIdeas:
      "Si vols, puc donar-te entre 1 i 3 idees d'optimització accionables basades en aquests resultats.",
    completionFollowupInspectFailedSteps:
      'Si vols, puc revisar els passos fallits i tornar-ho a provar amb una via de reserva més segura.',
    completionFollowupRerunValidation:
      "Si vols, puc tornar a executar la validació després de l'intent de recuperació per confirmar el resultat.",
    completionFollowupKeepFixing:
      "Si vols, puc continuar corregint els problemes restants o deixar l'informe final.",
    completionFollowupVerifyDeliverables:
      'Si vols, puc ajudar a verificar els lliurables al teu entorn.',
    completionFollowupRunRelevantTests:
      'Si vols, puc ajudar a executar les proves pertinents per confirmar que no hi ha regressions.',
    completionFollowupOptimizeNextArea:
      "Si vols, puc optimitzar la següent àrea que més t'importi.",
    completionFollowupIfYoudLikePrefix: 'Si vols, ',
    completionFollowupIfYouWantPrefix: 'Si vols, ',
  },
  'cs-CZ': {
    completionFollowupHeading: 'Pokud chcete, mohu vám také pomoci s:',
    completionFollowupNoFurtherActionNeeded: 'Není potřeba nic dalšího.',
    completionFollowupExpandFullerReport:
      'Pokud chcete, mohu to rozšířit do podrobnější zprávy.',
    completionFollowupVerifyKeyEvidence:
      'Pokud chcete, mohu pomoci ověřit klíčové důkazy v kartách nástrojů výše.',
    completionFollowupReorderTakeaways:
      'Pokud chcete, řekněte mi svou hlavní prioritu (například výkon/náklady/riziko) a seřadím pro vás hlavní závěry.',
    completionFollowupOptimizationIdeas:
      'Pokud chcete, mohu na základě těchto výsledků navrhnout 1-3 konkrétní optimalizační kroky.',
    completionFollowupInspectFailedSteps:
      'Pokud chcete, mohu prověřit selhané kroky a zopakovat pokus bezpečnější záložní cestou.',
    completionFollowupRerunValidation:
      'Pokud chcete, mohu po pokusu o obnovu znovu spustit ověření a potvrdit výsledek.',
    completionFollowupKeepFixing:
      'Pokud chcete, mohu pokračovat v opravě zbývajících problémů nebo dokončit zprávu.',
    completionFollowupVerifyDeliverables:
      'Pokud chcete, mohu pomoci ověřit výstupy ve vašem prostředí.',
    completionFollowupRunRelevantTests:
      'Pokud chcete, mohu pomoci spustit příslušné testy a potvrdit, že nedošlo k regresím.',
    completionFollowupOptimizeNextArea:
      'Pokud chcete, mohu optimalizovat další oblast, na které vám záleží.',
    completionFollowupIfYoudLikePrefix: 'Pokud chcete, ',
    completionFollowupIfYouWantPrefix: 'Pokud chcete, ',
  },
  'da-DK': {
    completionFollowupHeading: 'Hvis du vil, kan jeg også hjælpe med:',
    completionFollowupNoFurtherActionNeeded: 'Der er ikke behov for yderligere handling.',
    completionFollowupExpandFullerReport:
      'Hvis du vil, kan jeg udvide dette til en mere fyldig rapport.',
    completionFollowupVerifyKeyEvidence:
      'Hvis du vil, kan jeg hjælpe med at bekræfte det vigtigste bevis i værktøjskortene ovenfor.',
    completionFollowupReorderTakeaways:
      'Hvis du vil, så fortæl mig din vigtigste prioritet (for eksempel ydelse/omkostning/risiko), så kan jeg omprioritere hovedpunkterne for dig.',
    completionFollowupOptimizationIdeas:
      'Hvis du vil, kan jeg foreslå 1-3 konkrete optimeringsidéer baseret på disse resultater.',
    completionFollowupInspectFailedSteps:
      'Hvis du vil, kan jeg gennemgå de mislykkede trin og prøve igen med en sikrere reservevej.',
    completionFollowupRerunValidation:
      'Hvis du vil, kan jeg køre valideringen igen efter gendannelsesforsøget for at bekræfte resultatet.',
    completionFollowupKeepFixing:
      'Hvis du vil, kan jeg fortsætte med at rette de resterende problemer eller færdiggøre rapporten.',
    completionFollowupVerifyDeliverables:
      'Hvis du vil, kan jeg hjælpe med at bekræfte leverancerne i dit miljø.',
    completionFollowupRunRelevantTests:
      'Hvis du vil, kan jeg hjælpe med at køre de relevante tests for at bekræfte, at der ikke er regressioner.',
    completionFollowupOptimizeNextArea:
      'Hvis du vil, kan jeg optimere det næste område, du går mest op i.',
    completionFollowupIfYoudLikePrefix: 'Hvis du vil, ',
    completionFollowupIfYouWantPrefix: 'Hvis du vil, ',
  },
  'de-DE': {
    completionFollowupHeading:
      'Wenn Sie möchten, kann ich Ihnen auch bei Folgendem helfen:',
    completionFollowupNoFurtherActionNeeded: 'Es sind keine weiteren Schritte erforderlich.',
    completionFollowupExpandFullerReport:
      'Wenn Sie möchten, kann ich das zu einem ausführlicheren Bericht ausbauen.',
    completionFollowupVerifyKeyEvidence:
      'Wenn Sie möchten, kann ich helfen, die wichtigsten Belege in den Tool-Karten oben zu prüfen.',
    completionFollowupReorderTakeaways:
      'Wenn Sie möchten, nennen Sie mir Ihre wichtigste Priorität (zum Beispiel Leistung/Kosten/Risiko), und ich ordne die Kernaussagen für Sie neu.',
    completionFollowupOptimizationIdeas:
      'Wenn Sie möchten, kann ich auf Basis dieser Ergebnisse 1-3 konkrete Optimierungsideen vorschlagen.',
    completionFollowupInspectFailedSteps:
      'Wenn Sie möchten, kann ich die fehlgeschlagenen Schritte prüfen und es mit einem sichereren Fallback erneut versuchen.',
    completionFollowupRerunValidation:
      'Wenn Sie möchten, kann ich die Validierung nach dem Wiederherstellungsversuch erneut ausführen, um das Ergebnis zu bestätigen.',
    completionFollowupKeepFixing:
      'Wenn Sie möchten, kann ich die verbleibenden Probleme weiter beheben oder den Bericht fertigstellen.',
    completionFollowupVerifyDeliverables:
      'Wenn Sie möchten, kann ich helfen, die Ergebnisse in Ihrer Umgebung zu verifizieren.',
    completionFollowupRunRelevantTests:
      'Wenn Sie möchten, kann ich die relevanten Tests ausführen, um zu bestätigen, dass es keine Regressionen gibt.',
    completionFollowupOptimizeNextArea:
      'Wenn Sie möchten, kann ich den nächsten Bereich optimieren, der Ihnen wichtig ist.',
    completionFollowupIfYoudLikePrefix: 'Wenn Sie möchten, ',
    completionFollowupIfYouWantPrefix: 'Wenn Sie möchten, ',
  },
  'el-GR': {
    completionFollowupHeading: 'Αν θέλετε, μπορώ επίσης να βοηθήσω με:',
    completionFollowupNoFurtherActionNeeded: 'Δεν απαιτείται περαιτέρω ενέργεια.',
    completionFollowupExpandFullerReport:
      'Αν θέλετε, μπορώ να το επεκτείνω σε μια πιο πλήρη αναφορά.',
    completionFollowupVerifyKeyEvidence:
      'Αν θέλετε, μπορώ να βοηθήσω να επαληθεύσουμε τα βασικά τεκμήρια στις κάρτες εργαλείων παραπάνω.',
    completionFollowupReorderTakeaways:
      'Αν θέλετε, πείτε μου την κορυφαία προτεραιότητά σας (για παράδειγμα απόδοση/κόστος/κίνδυνος) και μπορώ να αναδιατάξω τα βασικά συμπεράσματα για εσάς.',
    completionFollowupOptimizationIdeas:
      'Αν θέλετε, μπορώ να προτείνω 1-3 εφαρμόσιμες ιδέες βελτιστοποίησης βάσει αυτών των αποτελεσμάτων.',
    completionFollowupInspectFailedSteps:
      'Αν θέλετε, μπορώ να ελέγξω τα αποτυχημένα βήματα και να δοκιμάσω ξανά με ασφαλέστερη εναλλακτική διαδρομή.',
    completionFollowupRerunValidation:
      'Αν θέλετε, μπορώ να εκτελέσω ξανά την επαλήθευση μετά την προσπάθεια αποκατάστασης για να επιβεβαιώσω το αποτέλεσμα.',
    completionFollowupKeepFixing:
      'Αν θέλετε, μπορώ να συνεχίσω να διορθώνω τα υπόλοιπα ζητήματα ή να ολοκληρώσω την αναφορά.',
    completionFollowupVerifyDeliverables:
      'Αν θέλετε, μπορώ να βοηθήσω να επαληθεύσουμε τα παραδοτέα στο περιβάλλον σας.',
    completionFollowupRunRelevantTests:
      'Αν θέλετε, μπορώ να βοηθήσω να εκτελέσουμε τους σχετικούς ελέγχους για να επιβεβαιώσουμε ότι δεν υπάρχουν παλινδρομήσεις.',
    completionFollowupOptimizeNextArea:
      'Αν θέλετε, μπορώ να βελτιστοποιήσω την επόμενη περιοχή που σας ενδιαφέρει.',
    completionFollowupIfYoudLikePrefix: 'Αν θέλετε, ',
    completionFollowupIfYouWantPrefix: 'Αν θέλετε, ',
  },
  'en-GB': {
    completionFollowupHeading: "If you'd like, I can also help with:",
    completionFollowupNoFurtherActionNeeded: 'No further action needed.',
    completionFollowupExpandFullerReport:
      "If you'd like, I can expand this into a fuller report.",
    completionFollowupVerifyKeyEvidence:
      "If you'd like, I can help verify the key evidence in the tool cards above.",
    completionFollowupReorderTakeaways:
      "If you'd like, tell me your top priority (for example performance/cost/risk), and I can reorder the takeaways for you.",
    completionFollowupOptimizationIdeas:
      'If you want, I can provide 1-3 actionable optimization ideas based on these results.',
    completionFollowupInspectFailedSteps:
      "If you'd like, I can inspect the failed steps and retry with a safer fallback path.",
    completionFollowupRerunValidation:
      'If you want, I can re-run validation after the recovery attempt to confirm the result.',
    completionFollowupKeepFixing:
      "If you'd like, I can keep fixing the remaining issues or finalise the report.",
    completionFollowupVerifyDeliverables:
      "If you'd like, I can help verify the deliverables in your environment.",
    completionFollowupRunRelevantTests:
      'If you want, I can help run the relevant tests to confirm there are no regressions.',
    completionFollowupOptimizeNextArea:
      "If you'd like, I can optimise the next area you care about.",
    completionFollowupIfYoudLikePrefix: "If you'd like, ",
    completionFollowupIfYouWantPrefix: 'If you want, ',
  },
  'en-US': {
    completionFollowupHeading: "If you'd like, I can also help with:",
    completionFollowupNoFurtherActionNeeded: 'No further action needed.',
    completionFollowupExpandFullerReport:
      "If you'd like, I can expand this into a fuller report.",
    completionFollowupVerifyKeyEvidence:
      "If you'd like, I can help verify the key evidence in the tool cards above.",
    completionFollowupReorderTakeaways:
      "If you'd like, tell me your top priority (for example performance/cost/risk), and I can reorder the takeaways for you.",
    completionFollowupOptimizationIdeas:
      'If you want, I can provide 1-3 actionable optimization ideas based on these results.',
    completionFollowupInspectFailedSteps:
      "If you'd like, I can inspect the failed steps and retry with a safer fallback path.",
    completionFollowupRerunValidation:
      'If you want, I can re-run validation after the recovery attempt to confirm the result.',
    completionFollowupKeepFixing:
      "If you'd like, I can keep fixing the remaining issues or finalize the report.",
    completionFollowupVerifyDeliverables:
      "If you'd like, I can help verify the deliverables in your environment.",
    completionFollowupRunRelevantTests:
      'If you want, I can help run the relevant tests to confirm there are no regressions.',
    completionFollowupOptimizeNextArea:
      "If you'd like, I can optimize the next area you care about.",
    completionFollowupIfYoudLikePrefix: "If you'd like, ",
    completionFollowupIfYouWantPrefix: 'If you want, ',
  },
  'es-ES': {
    completionFollowupHeading: 'Si quieres, también puedo ayudarte con:',
    completionFollowupNoFurtherActionNeeded: 'No hace falta ninguna otra acción.',
    completionFollowupExpandFullerReport:
      'Si quieres, puedo ampliar esto en un informe más completo.',
    completionFollowupVerifyKeyEvidence:
      'Si quieres, puedo ayudar a verificar la evidencia clave en las tarjetas de herramientas de arriba.',
    completionFollowupReorderTakeaways:
      'Si quieres, dime tu prioridad principal (por ejemplo, rendimiento/coste/riesgo) y puedo reordenar las conclusiones para ti.',
    completionFollowupOptimizationIdeas:
      'Si quieres, puedo ofrecerte entre 1 y 3 ideas de optimización accionables basadas en estos resultados.',
    completionFollowupInspectFailedSteps:
      'Si quieres, puedo revisar los pasos fallidos y reintentarlo con una vía de respaldo más segura.',
    completionFollowupRerunValidation:
      'Si quieres, puedo volver a ejecutar la validación después del intento de recuperación para confirmar el resultado.',
    completionFollowupKeepFixing:
      'Si quieres, puedo seguir corrigiendo los problemas restantes o dejar listo el informe final.',
    completionFollowupVerifyDeliverables:
      'Si quieres, puedo ayudar a verificar los entregables en tu entorno.',
    completionFollowupRunRelevantTests:
      'Si quieres, puedo ayudar a ejecutar las pruebas relevantes para confirmar que no hay regresiones.',
    completionFollowupOptimizeNextArea:
      'Si quieres, puedo optimizar la siguiente área que más te importa.',
    completionFollowupIfYoudLikePrefix: 'Si quieres, ',
    completionFollowupIfYouWantPrefix: 'Si quieres, ',
  },
  'fr-FR': {
    completionFollowupHeading: 'Si vous le souhaitez, je peux aussi vous aider avec :',
    completionFollowupNoFurtherActionNeeded: "Aucune autre action n'est nécessaire.",
    completionFollowupExpandFullerReport:
      'Si vous le souhaitez, je peux développer cela dans un rapport plus complet.',
    completionFollowupVerifyKeyEvidence:
      "Si vous le souhaitez, je peux aider à vérifier les éléments de preuve clés dans les cartes d'outils ci-dessus.",
    completionFollowupReorderTakeaways:
      'Si vous le souhaitez, indiquez-moi votre priorité principale (par exemple performance/coût/risque) et je peux réorganiser les points clés pour vous.',
    completionFollowupOptimizationIdeas:
      "Si vous le souhaitez, je peux proposer 1 à 3 idées d'optimisation concrètes à partir de ces résultats.",
    completionFollowupInspectFailedSteps:
      'Si vous le souhaitez, je peux examiner les étapes en échec et réessayer avec une solution de secours plus sûre.',
    completionFollowupRerunValidation:
      'Si vous le souhaitez, je peux relancer la validation après la tentative de récupération pour confirmer le résultat.',
    completionFollowupKeepFixing:
      'Si vous le souhaitez, je peux continuer à corriger les problèmes restants ou finaliser le rapport.',
    completionFollowupVerifyDeliverables:
      'Si vous le souhaitez, je peux aider à vérifier les livrables dans votre environnement.',
    completionFollowupRunRelevantTests:
      "Si vous le souhaitez, je peux aider à exécuter les tests pertinents pour confirmer qu'il n'y a pas de régressions.",
    completionFollowupOptimizeNextArea:
      "Si vous le souhaitez, je peux optimiser le prochain domaine qui vous importe le plus.",
    completionFollowupIfYoudLikePrefix: 'Si vous le souhaitez, ',
    completionFollowupIfYouWantPrefix: 'Si vous le souhaitez, ',
  },
  'ga-IE': {
    completionFollowupHeading: 'Más mian leat, is féidir liom cabhrú leat freisin le:',
    completionFollowupNoFurtherActionNeeded: 'Níl aon ghníomh eile ag teastáil.',
    completionFollowupExpandFullerReport:
      'Más mian leat, is féidir liom é seo a leathnú ina thuairisc níos iomláine.',
    completionFollowupVerifyKeyEvidence:
      'Más mian leat, is féidir liom cabhrú leis an bhfianaise thábhachtach sna cártaí uirlisí thuas a fhíorú.',
    completionFollowupReorderTakeaways:
      'Más mian leat, inis dom do phríomhthosaíocht (mar shampla feidhmíocht/costas/riosca) agus is féidir liom na príomhphointí a athordú duit.',
    completionFollowupOptimizationIdeas:
      'Más mian leat, is féidir liom 1-3 smaoineamh optamúcháin inghníomhaithe a mholadh bunaithe ar na torthaí seo.',
    completionFollowupInspectFailedSteps:
      'Más mian leat, is féidir liom na céimeanna ar theip orthu a scrúdú agus triail eile a bhaint as le cosán cúltaca níos sábháilte.',
    completionFollowupRerunValidation:
      'Más mian leat, is féidir liom an bailíochtú a rith arís tar éis na hiarrachta téarnaimh chun an toradh a dhearbhú.',
    completionFollowupKeepFixing:
      'Más mian leat, is féidir liom leanúint de na saincheisteanna atá fágtha a shocrú nó an tuairisc a chríochnú.',
    completionFollowupVerifyDeliverables:
      'Más mian leat, is féidir liom cabhrú leis na seachadtaí i do thimpeallacht a fhíorú.',
    completionFollowupRunRelevantTests:
      'Más mian leat, is féidir liom cabhrú leis na tástálacha ábhartha a rith chun a dhearbhú nach bhfuil aon aischéimnithe ann.',
    completionFollowupOptimizeNextArea:
      'Más mian leat, is féidir liom an chéad réimse eile is cúram duit a bharrfheabhsú.',
    completionFollowupIfYoudLikePrefix: 'Más mian leat, ',
    completionFollowupIfYouWantPrefix: 'Más mian leat, ',
  },
  'hr-HR': {
    completionFollowupHeading: 'Ako želite, mogu vam također pomoći s:',
    completionFollowupNoFurtherActionNeeded: 'Nisu potrebne daljnje radnje.',
    completionFollowupExpandFullerReport:
      'Ako želite, mogu ovo proširiti u potpunije izvješće.',
    completionFollowupVerifyKeyEvidence:
      'Ako želite, mogu pomoći provjeriti ključne dokaze u karticama alata iznad.',
    completionFollowupReorderTakeaways:
      'Ako želite, recite mi svoj glavni prioritet (na primjer izvedba/trošak/rizik) i mogu presložiti glavne zaključke za vas.',
    completionFollowupOptimizationIdeas:
      'Ako želite, mogu predložiti 1-3 provedive ideje za optimizaciju na temelju ovih rezultata.',
    completionFollowupInspectFailedSteps:
      'Ako želite, mogu pregledati neuspjele korake i pokušati ponovno sigurnijim rezervnim putem.',
    completionFollowupRerunValidation:
      'Ako želite, mogu ponovno pokrenuti provjeru nakon pokušaja oporavka kako bih potvrdio rezultat.',
    completionFollowupKeepFixing:
      'Ako želite, mogu nastaviti popravljati preostale probleme ili dovršiti izvješće.',
    completionFollowupVerifyDeliverables:
      'Ako želite, mogu pomoći provjeriti isporučene rezultate u vašem okruženju.',
    completionFollowupRunRelevantTests:
      'Ako želite, mogu pomoći pokrenuti relevantne testove kako bih potvrdio da nema regresija.',
    completionFollowupOptimizeNextArea:
      'Ako želite, mogu optimizirati sljedeće područje koje vam je važno.',
    completionFollowupIfYoudLikePrefix: 'Ako želite, ',
    completionFollowupIfYouWantPrefix: 'Ako želite, ',
  },
  'hu-HU': {
    completionFollowupHeading: 'Ha szeretné, ezekben is tudok segíteni:',
    completionFollowupNoFurtherActionNeeded: 'Nincs szükség további teendőre.',
    completionFollowupExpandFullerReport:
      'Ha szeretné, ezt kibővíthetem egy részletesebb jelentéssé.',
    completionFollowupVerifyKeyEvidence:
      'Ha szeretné, segíthetek ellenőrizni a fenti eszközkártyák kulcsfontosságú bizonyítékait.',
    completionFollowupReorderTakeaways:
      'Ha szeretné, mondja el a legfontosabb prioritását (például teljesítmény/költség/kockázat), és ennek megfelelően átrendezem a fő megállapításokat.',
    completionFollowupOptimizationIdeas:
      'Ha szeretné, ezen eredmények alapján 1-3 konkrét optimalizálási ötletet is javasolhatok.',
    completionFollowupInspectFailedSteps:
      'Ha szeretné, átnézhetem a sikertelen lépéseket, és újra megpróbálhatom egy biztonságosabb tartalék útvonallal.',
    completionFollowupRerunValidation:
      'Ha szeretné, a helyreállítási kísérlet után újra futtathatom az ellenőrzést az eredmény megerősítéséhez.',
    completionFollowupKeepFixing:
      'Ha szeretné, folytathatom a fennmaradó problémák javítását, vagy véglegesíthetem a jelentést.',
    completionFollowupVerifyDeliverables:
      'Ha szeretné, segíthetek ellenőrizni a leszállított eredményeket a környezetében.',
    completionFollowupRunRelevantTests:
      'Ha szeretné, segíthetek lefuttatni a releváns teszteket annak megerősítésére, hogy nincs regresszió.',
    completionFollowupOptimizeNextArea:
      'Ha szeretné, optimalizálhatom a következő területet, ami a leginkább érdekli.',
    completionFollowupIfYoudLikePrefix: 'Ha szeretné, ',
    completionFollowupIfYouWantPrefix: 'Ha szeretné, ',
  },
  'it-IT': {
    completionFollowupHeading: 'Se vuoi, posso anche aiutarti con:',
    completionFollowupNoFurtherActionNeeded: 'Non sono necessarie altre azioni.',
    completionFollowupExpandFullerReport:
      'Se vuoi, posso ampliare questo in un report più completo.',
    completionFollowupVerifyKeyEvidence:
      'Se vuoi, posso aiutare a verificare le prove chiave nelle schede degli strumenti qui sopra.',
    completionFollowupReorderTakeaways:
      'Se vuoi, dimmi la tua priorità principale (ad esempio prestazioni/costo/rischio) e posso riordinare i punti chiave per te.',
    completionFollowupOptimizationIdeas:
      'Se vuoi, posso proporti da 1 a 3 idee di ottimizzazione concrete basate su questi risultati.',
    completionFollowupInspectFailedSteps:
      'Se vuoi, posso esaminare i passaggi non riusciti e riprovare con un percorso di fallback più sicuro.',
    completionFollowupRerunValidation:
      'Se vuoi, posso rieseguire la validazione dopo il tentativo di recupero per confermare il risultato.',
    completionFollowupKeepFixing:
      'Se vuoi, posso continuare a correggere i problemi rimanenti o finalizzare il report.',
    completionFollowupVerifyDeliverables:
      'Se vuoi, posso aiutare a verificare i risultati consegnati nel tuo ambiente.',
    completionFollowupRunRelevantTests:
      'Se vuoi, posso aiutare a eseguire i test pertinenti per confermare che non ci sono regressioni.',
    completionFollowupOptimizeNextArea:
      'Se vuoi, posso ottimizzare l’area successiva che ti interessa di più.',
    completionFollowupIfYoudLikePrefix: 'Se vuoi, ',
    completionFollowupIfYouWantPrefix: 'Se vuoi, ',
  },
  'ja-JP': {
    completionFollowupHeading: '必要であれば、次のこともお手伝いできます:',
    completionFollowupNoFurtherActionNeeded: '現時点で追加の対応は不要です。',
    completionFollowupExpandFullerReport:
      '必要であれば、これをさらに詳しいレポートにまとめられます。',
    completionFollowupVerifyKeyEvidence:
      '必要であれば、上のツールカードにある重要な根拠を確認するお手伝いができます。',
    completionFollowupReorderTakeaways:
      '必要であれば、何を最優先にしたいか（例: 性能/コスト/リスク）を教えてください。要点を優先順に並べ替えます。',
    completionFollowupOptimizationIdeas:
      '必要であれば、これらの結果に基づいて実行しやすい最適化案を1〜3件提案できます。',
    completionFollowupInspectFailedSteps:
      '必要であれば、失敗した手順を確認し、より安全な代替経路で再試行できます。',
    completionFollowupRerunValidation:
      '必要であれば、復旧を試したあとに検証を再実行して結果を確認できます。',
    completionFollowupKeepFixing:
      '必要であれば、残っている問題の修正を続けるか、最終レポートを仕上げられます。',
    completionFollowupVerifyDeliverables:
      '必要であれば、あなたの環境で成果物を確認するお手伝いができます。',
    completionFollowupRunRelevantTests:
      '必要であれば、関連テストを実行して回帰がないことを確認できます。',
    completionFollowupOptimizeNextArea:
      '必要であれば、次に重視したい領域を最適化できます。',
    completionFollowupIfYoudLikePrefix: '必要であれば、',
    completionFollowupIfYouWantPrefix: '必要であれば、',
  },
  'ko-KR': {
    completionFollowupHeading: '원하시면 다음도 도와드릴 수 있습니다:',
    completionFollowupNoFurtherActionNeeded: '현재 추가로 할 일은 없습니다.',
    completionFollowupExpandFullerReport:
      '원하시면 이 내용을 더 자세한 보고서로 확장해 드릴 수 있습니다.',
    completionFollowupVerifyKeyEvidence:
      '원하시면 위 도구 카드의 핵심 근거를 함께 확인해 드릴 수 있습니다.',
    completionFollowupReorderTakeaways:
      '원하시면 가장 중요한 우선순위(예: 성능/비용/위험)를 알려 주세요. 그에 맞게 핵심 내용을 다시 정리해 드리겠습니다.',
    completionFollowupOptimizationIdeas:
      '원하시면 이 결과를 바탕으로 실행 가능한 최적화 아이디어 1-3가지를 제안해 드릴 수 있습니다.',
    completionFollowupInspectFailedSteps:
      '원하시면 실패한 단계를 점검하고 더 안전한 우회 경로로 다시 시도해 볼 수 있습니다.',
    completionFollowupRerunValidation:
      '원하시면 복구 시도 후 검증을 다시 실행해 결과를 확인해 드릴 수 있습니다.',
    completionFollowupKeepFixing:
      '원하시면 남은 문제를 계속 수정하거나 최종 보고서를 마무리해 드릴 수 있습니다.',
    completionFollowupVerifyDeliverables:
      '원하시면 사용자 환경에서 결과물을 함께 확인해 드릴 수 있습니다.',
    completionFollowupRunRelevantTests:
      '원하시면 관련 테스트를 실행해 회귀가 없는지 확인해 드릴 수 있습니다.',
    completionFollowupOptimizeNextArea:
      '원하시면 다음으로 중요하게 보는 영역을 최적화해 드릴 수 있습니다.',
    completionFollowupIfYoudLikePrefix: '원하시면 ',
    completionFollowupIfYouWantPrefix: '원하시면 ',
  },
  'ml-IN': {
    completionFollowupHeading: 'താൽപ്പര്യമുണ്ടെങ്കിൽ, ഇതിലും ഞാൻ സഹായിക്കാം:',
    completionFollowupNoFurtherActionNeeded: 'ഇപ്പോൾ കൂടുതൽ നടപടിയൊന്നും വേണ്ട.',
    completionFollowupExpandFullerReport:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, ഇതിനെ കൂടുതൽ സമഗ്രമായൊരു റിപ്പോർട്ടാക്കി വികസിപ്പിക്കാം.',
    completionFollowupVerifyKeyEvidence:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, മുകളിലുള്ള ടൂൾ കാർഡുകളിലെ പ്രധാന തെളിവുകൾ പരിശോധിക്കാൻ ഞാൻ സഹായിക്കാം.',
    completionFollowupReorderTakeaways:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, നിങ്ങളുടെ പ്രധാന മുൻഗണന (ഉദാഹരണം പ്രകടനം/ചെലവ്/റിസ്ക്) എന്നോട് പറയൂ; പ്രധാനപ്പെട്ട കാര്യങ്ങൾ അതനുസരിച്ച് വീണ്ടും ക്രമീകരിക്കാം.',
    completionFollowupOptimizationIdeas:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, ഈ ഫലങ്ങളെ അടിസ്ഥാനമാക്കി 1-3 പ്രായോഗിക ഒപ്റ്റിമൈസേഷൻ ആശയങ്ങൾ നിർദേശിക്കാം.',
    completionFollowupInspectFailedSteps:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, പരാജയപ്പെട്ട ഘട്ടങ്ങൾ പരിശോധിച്ച് കൂടുതൽ സുരക്ഷിതമായ fallback വഴിയിലൂടെ വീണ്ടും ശ്രമിക്കാം.',
    completionFollowupRerunValidation:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, വീണ്ടെടുപ്പ് ശ്രമത്തിനു ശേഷം ഫലം സ്ഥിരീകരിക്കാൻ വീണ്ടും സാധൂകരണം നടത്താം.',
    completionFollowupKeepFixing:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, ശേഷിക്കുന്ന പ്രശ്നങ്ങൾ പരിഹരിക്കുന്നത് തുടരുകയോ റിപ്പോർട്ട് അന്തിമമാക്കുകയോ ചെയ്യാം.',
    completionFollowupVerifyDeliverables:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, നിങ്ങളുടെ പരിസരത്തിലെ ഡെലിവറബിളുകൾ സ്ഥിരീകരിക്കാൻ ഞാൻ സഹായിക്കാം.',
    completionFollowupRunRelevantTests:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, regression ഇല്ലെന്ന് ഉറപ്പാക്കാൻ ബന്ധപ്പെട്ട ടെസ്റ്റുകൾ ഓടിക്കാൻ ഞാൻ സഹായിക്കാം.',
    completionFollowupOptimizeNextArea:
      'താൽപ്പര്യമുണ്ടെങ്കിൽ, നിങ്ങൾക്ക് പ്രധാനപ്പെട്ട അടുത്ത മേഖലയെ മെച്ചപ്പെടുത്താം.',
    completionFollowupIfYoudLikePrefix: 'താൽപ്പര്യമുണ്ടെങ്കിൽ, ',
    completionFollowupIfYouWantPrefix: 'താൽപ്പര്യമുണ്ടെങ്കിൽ, ',
  },
  'nb-NO': {
    completionFollowupHeading: 'Hvis du vil, kan jeg også hjelpe med:',
    completionFollowupNoFurtherActionNeeded: 'Ingen videre handling er nødvendig.',
    completionFollowupExpandFullerReport:
      'Hvis du vil, kan jeg utvide dette til en mer fullstendig rapport.',
    completionFollowupVerifyKeyEvidence:
      'Hvis du vil, kan jeg hjelpe med å verifisere hovedbevisene i verktøykortene ovenfor.',
    completionFollowupReorderTakeaways:
      'Hvis du vil, fortell meg hva som er høyest prioritet for deg (for eksempel ytelse/kostnad/risiko), så kan jeg omprioritere hovedpunktene.',
    completionFollowupOptimizationIdeas:
      'Hvis du vil, kan jeg foreslå 1-3 konkrete optimaliseringsidéer basert på disse resultatene.',
    completionFollowupInspectFailedSteps:
      'Hvis du vil, kan jeg gå gjennom de mislykkede trinnene og prøve igjen med en sikrere reservevei.',
    completionFollowupRerunValidation:
      'Hvis du vil, kan jeg kjøre valideringen på nytt etter gjenopprettingsforsøket for å bekrefte resultatet.',
    completionFollowupKeepFixing:
      'Hvis du vil, kan jeg fortsette å rette de gjenstående problemene eller ferdigstille rapporten.',
    completionFollowupVerifyDeliverables:
      'Hvis du vil, kan jeg hjelpe med å verifisere leveransene i miljøet ditt.',
    completionFollowupRunRelevantTests:
      'Hvis du vil, kan jeg hjelpe med å kjøre de relevante testene for å bekrefte at det ikke finnes regresjoner.',
    completionFollowupOptimizeNextArea:
      'Hvis du vil, kan jeg optimalisere det neste området du bryr deg om.',
    completionFollowupIfYoudLikePrefix: 'Hvis du vil, ',
    completionFollowupIfYouWantPrefix: 'Hvis du vil, ',
  },
  'nl-NL': {
    completionFollowupHeading: 'Als je wilt, kan ik ook helpen met:',
    completionFollowupNoFurtherActionNeeded: 'Er is geen verdere actie nodig.',
    completionFollowupExpandFullerReport:
      'Als je wilt, kan ik dit uitbreiden tot een uitgebreider rapport.',
    completionFollowupVerifyKeyEvidence:
      'Als je wilt, kan ik helpen de belangrijkste onderbouwing in de toolkaarten hierboven te verifiëren.',
    completionFollowupReorderTakeaways:
      'Als je wilt, vertel me dan je hoogste prioriteit (bijvoorbeeld prestaties/kosten/risico), dan kan ik de belangrijkste conclusies voor je herordenen.',
    completionFollowupOptimizationIdeas:
      'Als je wilt, kan ik 1-3 concrete optimalisatie-ideeën voorstellen op basis van deze resultaten.',
    completionFollowupInspectFailedSteps:
      'Als je wilt, kan ik de mislukte stappen bekijken en het opnieuw proberen via een veiliger terugvalpad.',
    completionFollowupRerunValidation:
      'Als je wilt, kan ik de validatie opnieuw uitvoeren na de herstelpoging om het resultaat te bevestigen.',
    completionFollowupKeepFixing:
      'Als je wilt, kan ik de resterende problemen blijven oplossen of het rapport afronden.',
    completionFollowupVerifyDeliverables:
      'Als je wilt, kan ik helpen de opleveringen in jouw omgeving te verifiëren.',
    completionFollowupRunRelevantTests:
      'Als je wilt, kan ik helpen de relevante tests uit te voeren om te bevestigen dat er geen regressies zijn.',
    completionFollowupOptimizeNextArea:
      'Als je wilt, kan ik het volgende gebied optimaliseren dat voor jou het belangrijkst is.',
    completionFollowupIfYoudLikePrefix: 'Als je wilt, ',
    completionFollowupIfYouWantPrefix: 'Als je wilt, ',
  },
  'pl-PL': {
    completionFollowupHeading: 'Jeśli chcesz, mogę też pomóc w:',
    completionFollowupNoFurtherActionNeeded: 'Nie są potrzebne żadne dalsze działania.',
    completionFollowupExpandFullerReport:
      'Jeśli chcesz, mogę rozwinąć to w pełniejszy raport.',
    completionFollowupVerifyKeyEvidence:
      'Jeśli chcesz, mogę pomóc zweryfikować kluczowe dowody na kartach narzędzi powyżej.',
    completionFollowupReorderTakeaways:
      'Jeśli chcesz, powiedz mi, co jest dla Ciebie najważniejsze (na przykład wydajność/koszt/ryzyko), a uporządkuję wnioski według priorytetu.',
    completionFollowupOptimizationIdeas:
      'Jeśli chcesz, mogę zaproponować 1-3 praktyczne pomysły optymalizacyjne na podstawie tych wyników.',
    completionFollowupInspectFailedSteps:
      'Jeśli chcesz, mogę przejrzeć nieudane kroki i spróbować ponownie bezpieczniejszą ścieżką awaryjną.',
    completionFollowupRerunValidation:
      'Jeśli chcesz, mogę ponownie uruchomić walidację po próbie odzyskania, aby potwierdzić wynik.',
    completionFollowupKeepFixing:
      'Jeśli chcesz, mogę dalej naprawiać pozostałe problemy albo sfinalizować raport.',
    completionFollowupVerifyDeliverables:
      'Jeśli chcesz, mogę pomóc zweryfikować rezultaty w Twoim środowisku.',
    completionFollowupRunRelevantTests:
      'Jeśli chcesz, mogę pomóc uruchomić odpowiednie testy, aby potwierdzić brak regresji.',
    completionFollowupOptimizeNextArea:
      'Jeśli chcesz, mogę zoptymalizować kolejny obszar, który jest dla Ciebie ważny.',
    completionFollowupIfYoudLikePrefix: 'Jeśli chcesz, ',
    completionFollowupIfYouWantPrefix: 'Jeśli chcesz, ',
  },
  'pt-BR': {
    completionFollowupHeading: 'Se quiser, também posso ajudar com:',
    completionFollowupNoFurtherActionNeeded: 'Nenhuma ação adicional é necessária.',
    completionFollowupExpandFullerReport:
      'Se quiser, posso expandir isto em um relatório mais completo.',
    completionFollowupVerifyKeyEvidence:
      'Se quiser, posso ajudar a verificar as evidências principais nos cartões de ferramentas acima.',
    completionFollowupReorderTakeaways:
      'Se quiser, diga qual é sua principal prioridade (por exemplo desempenho/custo/risco) e eu posso reorganizar os principais pontos para você.',
    completionFollowupOptimizationIdeas:
      'Se quiser, posso sugerir de 1 a 3 ideias de otimização acionáveis com base nesses resultados.',
    completionFollowupInspectFailedSteps:
      'Se quiser, posso revisar as etapas que falharam e tentar novamente com um caminho de fallback mais seguro.',
    completionFollowupRerunValidation:
      'Se quiser, posso executar a validação novamente após a tentativa de recuperação para confirmar o resultado.',
    completionFollowupKeepFixing:
      'Se quiser, posso continuar corrigindo os problemas restantes ou finalizar o relatório.',
    completionFollowupVerifyDeliverables:
      'Se quiser, posso ajudar a verificar as entregas no seu ambiente.',
    completionFollowupRunRelevantTests:
      'Se quiser, posso ajudar a executar os testes relevantes para confirmar que não há regressões.',
    completionFollowupOptimizeNextArea:
      'Se quiser, posso otimizar a próxima área que mais importa para você.',
    completionFollowupIfYoudLikePrefix: 'Se quiser, ',
    completionFollowupIfYouWantPrefix: 'Se quiser, ',
  },
  'pt-PT': {
    completionFollowupHeading: 'Se quiser, também posso ajudar com:',
    completionFollowupNoFurtherActionNeeded: 'Nenhuma ação adicional é necessária.',
    completionFollowupExpandFullerReport:
      'Se quiser, posso expandir isto num relatório mais completo.',
    completionFollowupVerifyKeyEvidence:
      'Se quiser, posso ajudar a verificar as evidências principais nos cartões de ferramentas acima.',
    completionFollowupReorderTakeaways:
      'Se quiser, diga qual é a sua principal prioridade (por exemplo desempenho/custo/risco) e eu posso reorganizar os pontos principais para si.',
    completionFollowupOptimizationIdeas:
      'Se quiser, posso sugerir de 1 a 3 ideias de otimização acionáveis com base nestes resultados.',
    completionFollowupInspectFailedSteps:
      'Se quiser, posso rever as etapas que falharam e tentar novamente com um caminho de fallback mais seguro.',
    completionFollowupRerunValidation:
      'Se quiser, posso executar a validação novamente após a tentativa de recuperação para confirmar o resultado.',
    completionFollowupKeepFixing:
      'Se quiser, posso continuar a corrigir os problemas restantes ou finalizar o relatório.',
    completionFollowupVerifyDeliverables:
      'Se quiser, posso ajudar a verificar as entregas no seu ambiente.',
    completionFollowupRunRelevantTests:
      'Se quiser, posso ajudar a executar os testes relevantes para confirmar que não há regressões.',
    completionFollowupOptimizeNextArea:
      'Se quiser, posso otimizar a próxima área que mais importa para si.',
    completionFollowupIfYoudLikePrefix: 'Se quiser, ',
    completionFollowupIfYouWantPrefix: 'Se quiser, ',
  },
  'ro-RO': {
    completionFollowupHeading: 'Dacă doriți, vă mai pot ajuta și cu:',
    completionFollowupNoFurtherActionNeeded: 'Nu este necesară nicio altă acțiune.',
    completionFollowupExpandFullerReport:
      'Dacă doriți, pot extinde aceasta într-un raport mai complet.',
    completionFollowupVerifyKeyEvidence:
      'Dacă doriți, pot ajuta la verificarea dovezilor cheie din cardurile de instrumente de mai sus.',
    completionFollowupReorderTakeaways:
      'Dacă doriți, spuneți-mi care este prioritatea dumneavoastră principală (de exemplu performanță/cost/risc) și pot rearanja concluziile principale pentru dumneavoastră.',
    completionFollowupOptimizationIdeas:
      'Dacă doriți, pot propune 1-3 idei de optimizare ușor de pus în practică pe baza acestor rezultate.',
    completionFollowupInspectFailedSteps:
      'Dacă doriți, pot examina pașii eșuați și pot încerca din nou cu o cale de rezervă mai sigură.',
    completionFollowupRerunValidation:
      'Dacă doriți, pot rula din nou validarea după încercarea de recuperare pentru a confirma rezultatul.',
    completionFollowupKeepFixing:
      'Dacă doriți, pot continua să repar problemele rămase sau pot finaliza raportul.',
    completionFollowupVerifyDeliverables:
      'Dacă doriți, pot ajuta la verificarea livrabilelor în mediul dumneavoastră.',
    completionFollowupRunRelevantTests:
      'Dacă doriți, pot ajuta la rularea testelor relevante pentru a confirma că nu există regresii.',
    completionFollowupOptimizeNextArea:
      'Dacă doriți, pot optimiza următoarea zonă care vă interesează.',
    completionFollowupIfYoudLikePrefix: 'Dacă doriți, ',
    completionFollowupIfYouWantPrefix: 'Dacă doriți, ',
  },
  'ru-RU': {
    completionFollowupHeading: 'Если хотите, я также могу помочь с:',
    completionFollowupNoFurtherActionNeeded: 'Дополнительных действий не требуется.',
    completionFollowupExpandFullerReport:
      'Если хотите, я могу развернуть это в более подробный отчёт.',
    completionFollowupVerifyKeyEvidence:
      'Если хотите, я могу помочь проверить ключевые подтверждения в карточках инструментов выше.',
    completionFollowupReorderTakeaways:
      'Если хотите, скажите, что для вас в приоритете (например, производительность/стоимость/риск), и я переставлю основные выводы по важности.',
    completionFollowupOptimizationIdeas:
      'Если хотите, я могу предложить 1-3 практичных идеи по оптимизации на основе этих результатов.',
    completionFollowupInspectFailedSteps:
      'Если хотите, я могу проверить неудавшиеся шаги и повторить попытку по более безопасному резервному сценарию.',
    completionFollowupRerunValidation:
      'Если хотите, я могу повторно запустить проверку после попытки восстановления, чтобы подтвердить результат.',
    completionFollowupKeepFixing:
      'Если хотите, я могу продолжить исправлять оставшиеся проблемы или завершить отчёт.',
    completionFollowupVerifyDeliverables:
      'Если хотите, я могу помочь проверить результаты в вашей среде.',
    completionFollowupRunRelevantTests:
      'Если хотите, я могу помочь запустить нужные тесты, чтобы убедиться, что регрессий нет.',
    completionFollowupOptimizeNextArea:
      'Если хотите, я могу оптимизировать следующую область, которая для вас важна.',
    completionFollowupIfYoudLikePrefix: 'Если хотите, ',
    completionFollowupIfYouWantPrefix: 'Если хотите, ',
  },
  'sk-SK': {
    completionFollowupHeading: 'Ak chcete, môžem vám tiež pomôcť s:',
    completionFollowupNoFurtherActionNeeded: 'Nie sú potrebné žiadne ďalšie kroky.',
    completionFollowupExpandFullerReport:
      'Ak chcete, môžem to rozšíriť na podrobnejšiu správu.',
    completionFollowupVerifyKeyEvidence:
      'Ak chcete, môžem pomôcť overiť kľúčové dôkazy v kartách nástrojov vyššie.',
    completionFollowupReorderTakeaways:
      'Ak chcete, povedzte mi svoju hlavnú prioritu (napríklad výkon/náklady/riziko) a zoradím hlavné zistenia podľa nej.',
    completionFollowupOptimizationIdeas:
      'Ak chcete, môžem navrhnúť 1-3 konkrétne optimalizačné nápady na základe týchto výsledkov.',
    completionFollowupInspectFailedSteps:
      'Ak chcete, môžem skontrolovať zlyhané kroky a skúsiť to znova bezpečnejšou záložnou cestou.',
    completionFollowupRerunValidation:
      'Ak chcete, môžem po pokuse o obnovu znova spustiť validáciu a potvrdiť výsledok.',
    completionFollowupKeepFixing:
      'Ak chcete, môžem pokračovať v opravovaní zostávajúcich problémov alebo dokončiť správu.',
    completionFollowupVerifyDeliverables:
      'Ak chcete, môžem pomôcť overiť výstupy vo vašom prostredí.',
    completionFollowupRunRelevantTests:
      'Ak chcete, môžem pomôcť spustiť relevantné testy a potvrdiť, že nedošlo k regresiám.',
    completionFollowupOptimizeNextArea:
      'Ak chcete, môžem optimalizovať ďalšiu oblasť, na ktorej vám záleží.',
    completionFollowupIfYoudLikePrefix: 'Ak chcete, ',
    completionFollowupIfYouWantPrefix: 'Ak chcete, ',
  },
  'sv-SE': {
    completionFollowupHeading: 'Om du vill kan jag också hjälpa till med:',
    completionFollowupNoFurtherActionNeeded: 'Ingen ytterligare åtgärd behövs.',
    completionFollowupExpandFullerReport:
      'Om du vill kan jag utveckla detta till en mer fullständig rapport.',
    completionFollowupVerifyKeyEvidence:
      'Om du vill kan jag hjälpa till att verifiera de viktigaste bevisen i verktygskorten ovan.',
    completionFollowupReorderTakeaways:
      'Om du vill, berätta vad som är högst prioritet för dig (till exempel prestanda/kostnad/risk), så kan jag ordna om slutsatserna efter det.',
    completionFollowupOptimizationIdeas:
      'Om du vill kan jag föreslå 1-3 konkreta optimeringsidéer utifrån dessa resultat.',
    completionFollowupInspectFailedSteps:
      'Om du vill kan jag granska de misslyckade stegen och försöka igen med en säkrare reservväg.',
    completionFollowupRerunValidation:
      'Om du vill kan jag köra valideringen igen efter återställningsförsöket för att bekräfta resultatet.',
    completionFollowupKeepFixing:
      'Om du vill kan jag fortsätta att åtgärda de återstående problemen eller slutföra rapporten.',
    completionFollowupVerifyDeliverables:
      'Om du vill kan jag hjälpa till att verifiera leveranserna i din miljö.',
    completionFollowupRunRelevantTests:
      'Om du vill kan jag hjälpa till att köra de relevanta testerna för att bekräfta att det inte finns några regressioner.',
    completionFollowupOptimizeNextArea:
      'Om du vill kan jag optimera nästa område som är viktigast för dig.',
    completionFollowupIfYoudLikePrefix: 'Om du vill, ',
    completionFollowupIfYouWantPrefix: 'Om du vill, ',
  },
  'zh-CN': {
    completionFollowupHeading: '如果你愿意，我还可以帮你：',
    completionFollowupNoFurtherActionNeeded: '当前无需进一步操作。',
    completionFollowupExpandFullerReport: '如果你愿意，我可以继续把这份结果扩展成更完整的总结。',
    completionFollowupVerifyKeyEvidence:
      '如果你愿意，我可以先帮你核对上方工具卡片里的关键信息。',
    completionFollowupReorderTakeaways:
      '如果你愿意，告诉我你最关心的方向（例如性能/成本/风险），我可以按优先级帮你重排。',
    completionFollowupOptimizationIdeas:
      '如果你希望，我也可以基于这些结果再给你 1-3 条可执行的优化建议。',
    completionFollowupInspectFailedSteps:
      '如果你愿意，我可以检查失败的步骤，并用更稳妥的兜底路径再试一次。',
    completionFollowupRerunValidation:
      '如果你希望，我也可以在恢复尝试后重新执行验证，确认结果是否成立。',
    completionFollowupKeepFixing:
      '如果你愿意，我可以继续处理剩余问题，或者帮你整理最终报告。',
    completionFollowupVerifyDeliverables:
      '如果你愿意，我可以帮你在你的环境里核对交付结果。',
    completionFollowupRunRelevantTests:
      '如果你希望，我也可以帮你运行相关测试，确认没有回归问题。',
    completionFollowupOptimizeNextArea:
      '如果你愿意，我可以继续优化你最关心的下一块内容。',
    completionFollowupIfYoudLikePrefix: '如果你愿意，',
    completionFollowupIfYouWantPrefix: '如果你希望，',
  },
  'zh-TW': {
    completionFollowupHeading: '如果你願意，我還可以幫你：',
    completionFollowupNoFurtherActionNeeded: '目前無需進一步操作。',
    completionFollowupExpandFullerReport: '如果你願意，我可以繼續把這份結果擴展成更完整的總結。',
    completionFollowupVerifyKeyEvidence:
      '如果你願意，我可以先幫你核對上方工具卡片裡的關鍵資訊。',
    completionFollowupReorderTakeaways:
      '如果你願意，告訴我你最在意的方向（例如效能/成本/風險），我可以按優先順序幫你重排。',
    completionFollowupOptimizationIdeas:
      '如果你希望，我也可以根據這些結果再給你 1-3 條可執行的優化建議。',
    completionFollowupInspectFailedSteps:
      '如果你願意，我可以檢查失敗的步驟，並用更穩妥的備援路徑再試一次。',
    completionFollowupRerunValidation:
      '如果你希望，我也可以在恢復嘗試後重新執行驗證，確認結果是否成立。',
    completionFollowupKeepFixing:
      '如果你願意，我可以繼續處理剩餘問題，或者幫你整理最終報告。',
    completionFollowupVerifyDeliverables:
      '如果你願意，我可以幫你在你的環境裡核對交付結果。',
    completionFollowupRunRelevantTests:
      '如果你希望，我也可以幫你執行相關測試，確認沒有回歸問題。',
    completionFollowupOptimizeNextArea:
      '如果你願意，我可以繼續優化你最關心的下一塊內容。',
    completionFollowupIfYoudLikePrefix: '如果你願意，',
    completionFollowupIfYouWantPrefix: '如果你希望，',
  },
}

const completionFollowupBackfills = Object.fromEntries(
  Object.entries(completionFollowupCopies).map(([localeKey, copy]) => [
    localeKey,
    buildCompletionFollowupBackfill(copy as CompletionFollowupChatMessages),
  ])
) as Partial<Record<LocaleKey, object>>

export function getCompletionFollowupMessagesForHeading(
  localizedHeading: string
): CompletionFollowupChatMessages | null {
  for (const copy of Object.values(completionFollowupCopies)) {
    if (copy?.completionFollowupHeading === localizedHeading) {
      return copy
    }
  }
  return null
}

export default completionFollowupBackfills
