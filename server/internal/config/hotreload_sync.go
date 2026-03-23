package config

import "log"

// SyncHotReloadToStore persists successful YAML reloads back into the kv-backed
// config store so /config/reload and file watching update the effective config.
func SyncHotReloadToStore(hr *HotReloader, store *ConfigStore) {
	if hr == nil || store == nil {
		return
	}

	hr.OnReload(func(event ReloadEvent) {
		if !event.Success || event.NewConfig == nil {
			return
		}
		if err := store.Import(event.NewConfig); err != nil {
			log.Printf("[ERROR] Failed to persist reloaded config into config store: %v", err)
			return
		}
		log.Printf("[INFO] Reloaded config persisted into config store")
	})
}
