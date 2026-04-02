package bootstrap

import (
	"reflect"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type runtimeGatewayContractSurface interface {
	HarnessRuntime() *HarnessRuntimeBundle
	ResearchService() *deepresearch.Service
	ReflectService() *selfreflect.Service
	RegisterTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration
	ApprovalDetailTarget() approvalRuntimeDetailTarget
}

var _ runtimeGatewayContractSurface = (*runtimeContractRuntimeBundle)(nil)

func TestRuntimeContract_GatewayAlignment(t *testing.T) {
	contractType := reflect.TypeOf((*runtimeGatewayContractSurface)(nil)).Elem()
	bundleType := reflect.TypeOf((*runtimeContractRuntimeBundle)(nil))

	if err := assertMethodSubset(contractType, bundleType); err != nil {
		t.Fatalf("runtime contract drift detected: %v", err)
	}
	if err := assertMethodSubset(reflect.TypeOf((*routeRuntimeResearchSurface)(nil)).Elem(), bundleType); err != nil {
		t.Fatalf("route runtime research surface drift detected: %v", err)
	}
}

func TestContract_AllowsAdditiveChanges(t *testing.T) {
	type additiveSurface interface {
		HarnessRuntime() *HarnessRuntimeBundle
	}
	if err := assertMethodSubset(reflect.TypeOf((*additiveSurface)(nil)).Elem(), reflect.TypeOf((*runtimeGatewayContractSurface)(nil)).Elem()); err != nil {
		t.Fatalf("expected additive subset check to pass: %v", err)
	}
	if err := assertSyntheticMethodSet(nil, map[string]reflect.Type{
		"HarnessRuntime": reflect.TypeOf(func() *HarnessRuntimeBundle { return nil }),
		"ExtraMethod":    reflect.TypeOf(func() {}),
	}, reflect.TypeOf((*additiveSurface)(nil)).Elem()); err != nil {
		t.Fatalf("expected additive synthetic contract to pass: %v", err)
	}
}

func TestContract_RejectsBreakingChanges(t *testing.T) {
	type breakingSurface interface {
		HarnessRuntime() *HarnessRuntimeBundle
	}
	err := assertSyntheticMethodSet(nil, map[string]reflect.Type{
		"HarnessRuntime": reflect.TypeOf(func() *deepresearch.Service { return nil }),
	}, reflect.TypeOf((*breakingSurface)(nil)).Elem())
	if err == nil {
		t.Fatal("expected breaking method signature drift to fail")
	}
}

func assertMethodSubset(contractType, implType reflect.Type) error {
	for i := 0; i < contractType.NumMethod(); i++ {
		contractMethod := contractType.Method(i)
		implMethod, ok := implType.MethodByName(contractMethod.Name)
		if !ok {
			return newContractDriftError(contractMethod.Name, "missing")
		}
		if !sameMethodShape(contractMethod.Type, implMethod.Type) {
			return newContractDriftError(contractMethod.Name, "signature_mismatch")
		}
	}
	return nil
}

func assertSyntheticMethodSet(_ reflect.Type, methods map[string]reflect.Type, contractType reflect.Type) error {
	for i := 0; i < contractType.NumMethod(); i++ {
		contractMethod := contractType.Method(i)
		implType, ok := methods[contractMethod.Name]
		if !ok {
			return newContractDriftError(contractMethod.Name, "missing")
		}
		if !sameSyntheticMethodShape(contractMethod.Type, implType) {
			return newContractDriftError(contractMethod.Name, "signature_mismatch")
		}
	}
	return nil
}

func sameMethodShape(contractType, implType reflect.Type) bool {
	implInputOffset := 0
	if implType.NumIn() == contractType.NumIn()+1 {
		implInputOffset = 1
	}
	if contractType.NumIn()+implInputOffset != implType.NumIn() || contractType.NumOut() != implType.NumOut() {
		return false
	}
	for i := 0; i < contractType.NumIn(); i++ {
		if contractType.In(i) != implType.In(i+implInputOffset) {
			return false
		}
	}
	for i := 0; i < contractType.NumOut(); i++ {
		if contractType.Out(i) != implType.Out(i) {
			return false
		}
	}
	return true
}

func sameSyntheticMethodShape(contractType, implType reflect.Type) bool {
	if implType.Kind() != reflect.Func {
		return false
	}
	if contractType.NumOut() != implType.NumOut() {
		return false
	}
	for i := 0; i < contractType.NumOut(); i++ {
		if contractType.Out(i) != implType.Out(i) {
			return false
		}
	}
	return true
}

func newContractDriftError(methodName, reason string) error {
	return &contractDriftError{MethodName: methodName, Reason: reason}
}

type contractDriftError struct {
	MethodName string
	Reason     string
}

func (e *contractDriftError) Error() string {
	if e == nil {
		return ""
	}
	return "runtime contract drift: " + e.MethodName + " (" + e.Reason + ")"
}
