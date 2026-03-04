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

// environmentBucketsToTF converts an EnvironmentObjectStorageConfig proto to a types.Object.
// Returns types.ObjectNull when b is nil.
func environmentBucketsToTF(b *serverv1.EnvironmentObjectStorageConfig) types.Object {
	attrTypes := map[string]attr.Type{
		"dataset_bucket":        types.StringType,
		"plan_stages_bucket":    types.StringType,
		"source_bundle_bucket":  types.StringType,
		"model_registry_bucket": types.StringType,
	}
	if b == nil {
		return types.ObjectNull(attrTypes)
	}
	return types.ObjectValueMust(attrTypes, map[string]attr.Value{
		"dataset_bucket":        optionalStringValue(b.DatasetBucket),
		"plan_stages_bucket":    optionalStringValue(b.PlanStagesBucket),
		"source_bundle_bucket":  optionalStringValue(b.SourceBundleBucket),
		"model_registry_bucket": optionalStringValue(b.ModelRegistryBucket),
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
