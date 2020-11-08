package util

import (
	"crypto/sha256"
	"encoding/base64"

	"k8s.io/kubernetes/pkg/util/hash"
)

type Annotated interface {
	GetAnnotations() map[string]string
	SetAnnotations(annotations map[string]string)
}

const resourceHashAnnotationKey = "cassandra-reaper.io/resource-hash"

func AddHashAnnotation(obj Annotated) {
	hash := deepHashString(obj)
	m := obj.GetAnnotations()
	if m == nil {
		m = map[string]string{}
	}
	m[resourceHashAnnotationKey] = hash
	obj.SetAnnotations(m)
}

func ResourcesHaveSameHash(r1, r2 Annotated) bool {
	a1 := r1.GetAnnotations()
	a2 := r2.GetAnnotations()
	if a1 == nil || a2 == nil {
		return false
	}
	return a1[resourceHashAnnotationKey] == a2[resourceHashAnnotationKey]
}

func deepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}
