package flux

import (
    "testing"
)

func TestParseResourceID(t *testing.T) {
    resourceId, err := ParseResourceID("default:customresourcedefinition/my.resource.com")
    if err != nil {
        t.Error("Got unexpected error", err)
    }

    if resourceId.String() != "default:customresourcedefinition/my.resource.com" {
        t.Error("Got unexpected resourceId", resourceId)
    }
}

func TestParseResourceIDOptionalNamespace(t *testing.T) {
    resourceId, err := ParseResourceIDOptionalNamespace("default", "test:customresourcedefinition/my.resource.com")
    if err != nil {
        t.Error("Got unexpected error", err)
    }

    if resourceId.String() != "test:customresourcedefinition/my.resource.com" {
        t.Error("Got unexpected resourceId", resourceId)
    }
}

func TestParseResourceIDOptionalNamespaceNotSet(t *testing.T) {
    resourceId, err := ParseResourceIDOptionalNamespace("default", "customresourcedefinition/my.resource.com")
    if err != nil {
        t.Error("Got unexpected error", err)
    }

    if resourceId.String() != "default:customresourcedefinition/my.resource.com" {
        t.Error("Got unexpected resourceId", resourceId)
    }
}
