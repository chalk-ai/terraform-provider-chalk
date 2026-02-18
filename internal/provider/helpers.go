package provider

import (
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// optionalStringValue converts an empty string to a null types.String, and a non-empty
// string to a types.StringValue. This prevents spurious drift when the server returns
// empty strings for fields the user did not configure.
func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
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
