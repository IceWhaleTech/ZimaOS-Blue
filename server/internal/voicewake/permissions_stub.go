//go:build !darwin

package voicewake

func defaultProbePermissionStatus() permissionStatus {
	return permissionStatus{}
}
