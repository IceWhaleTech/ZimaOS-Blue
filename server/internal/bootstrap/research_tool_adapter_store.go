package bootstrap

func (a *deepResearchToolAdapter) researchJobStore() researchHarnessJobStore {
	if a == nil || (a.jobStore == nil && a.service == nil) {
		return nil
	}
	if a.jobStore != nil {
		return a.jobStore
	}
	return a.service
}
