package provider

import (
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// validDeploymentBuildProfiles returns all known non-UNSPECIFIED DeploymentBuildProfile enum names.
func validDeploymentBuildProfiles() []string {
	var names []string
	for name, v := range serverv1.DeploymentBuildProfile_value {
		if serverv1.DeploymentBuildProfile(v) != serverv1.DeploymentBuildProfile_DEPLOYMENT_BUILD_PROFILE_UNSPECIFIED {
			names = append(names, name)
		}
	}
	return names
}

// stringSliceToListValue converts a []string to a types.List of strings.
// Returns types.ListNull when the slice is empty.
func stringSliceToListValue(items []string) types.List {
	if len(items) == 0 {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, len(items))
	for i, item := range items {
		elems[i] = types.StringValue(item)
	}
	return types.ListValueMust(types.StringType, elems)
}

// optionalStringValue converts an empty string to a null types.String, and a non-empty
// string to a types.StringValue. This prevents spurious drift when the server returns
// empty strings for fields the user did not configure.
func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// stringPointerValue converts an optional (nullable) proto string to a
// types.String using presence rather than emptiness: a nil pointer becomes null
// and any non-nil pointer (including "") is preserved. Pair it with
// types.String.ValueStringPointer() on the write side so a field round-trips
// exactly, distinguishing "unset" from "explicitly empty". Only safe for proto
// `optional string` fields whose presence the server round-trips faithfully; use
// optionalStringValue when the server may return "" for unset fields.
func stringPointerValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// boolPointerValue converts an optional (nullable) proto bool to a types.Bool
// using presence: a nil pointer becomes null. Pair it with
// types.Bool.ValueBoolPointer() so "unset" survives a round trip.
func boolPointerValue(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

// kubeResourceConfigObject converts a KubeResourceConfig proto to a types.Object.
func kubeResourceConfigObject(rc *serverv1.KubeResourceConfig) types.Object {
	if rc == nil {
		return types.ObjectNull(kubeResourceConfigAttrTypes)
	}
	return types.ObjectValueMust(kubeResourceConfigAttrTypes, map[string]attr.Value{
		"cpu":               optionalStringValue(rc.Cpu),
		"memory":            optionalStringValue(rc.Memory),
		"ephemeral_storage": optionalStringValue(rc.EphemeralStorage),
		"storage":           optionalStringValue(rc.Storage),
	})
}

// kubePVCObject converts a KubePersistentVolumeClaim proto to a types.Object.
func kubePVCObject(pvc *serverv1.KubePersistentVolumeClaim) types.Object {
	if pvc == nil {
		return types.ObjectNull(kubePVCAttrTypes)
	}
	return types.ObjectValueMust(kubePVCAttrTypes, map[string]attr.Value{
		"storage":            optionalStringValue(pvc.Storage),
		"storage_class_name": optionalStringValue(pvc.StorageClassName),
	})
}
