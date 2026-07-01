package provider

import (
	"fmt"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
)

// cloudStorageRoleValues is the set of friendly role strings accepted by the
// binding resources, in the order they are documented.
var cloudStorageRoleValues = []string{
	"DATASET",
	"PLAN_STAGES",
	"SOURCE_BUNDLE",
	"MODEL_REGISTRY",
	"VOLUME",
}

// friendlyToCloudStorageRole maps a friendly role string to its proto enum value.
var friendlyToCloudStorageRole = map[string]serverv1.CloudStorageRole{
	"DATASET":        serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET,
	"PLAN_STAGES":    serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES,
	"SOURCE_BUNDLE":  serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE,
	"MODEL_REGISTRY": serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY,
	"VOLUME":         serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME,
}

// cloudStorageRoleToFriendly is the inverse of friendlyToCloudStorageRole.
var cloudStorageRoleToFriendly = map[serverv1.CloudStorageRole]string{
	serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_DATASET:        "DATASET",
	serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_PLAN_STAGES:    "PLAN_STAGES",
	serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_SOURCE_BUNDLE:  "SOURCE_BUNDLE",
	serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_MODEL_REGISTRY: "MODEL_REGISTRY",
	serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_VOLUME:         "VOLUME",
}

// parseCloudStorageRole converts a friendly role string to its proto enum value.
func parseCloudStorageRole(friendly string) (serverv1.CloudStorageRole, error) {
	if role, ok := friendlyToCloudStorageRole[friendly]; ok {
		return role, nil
	}
	return serverv1.CloudStorageRole_CLOUD_STORAGE_ROLE_UNSPECIFIED, fmt.Errorf("unknown storage role %q (expected one of DATASET, PLAN_STAGES, SOURCE_BUNDLE, MODEL_REGISTRY, VOLUME)", friendly)
}

// cloudStorageRoleMarkdown is the shared schema description for the storage_role attribute.
const cloudStorageRoleMarkdown = "The role this storage fills for the target. One of `DATASET`, `PLAN_STAGES`, `SOURCE_BUNDLE`, `MODEL_REGISTRY`, or `VOLUME`. " +
	"A target may have at most one binding per role. Changing this forces a new resource."
