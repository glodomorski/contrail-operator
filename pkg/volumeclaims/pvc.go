package volumeclaims

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PersistentVolumeClaim interface {
	SetStoragePath(path string)
	SetStorageSize(quantity resource.Quantity)
	SetNodeSelector(nodeSelectors map[string]string)
	EnsureExists() error
}

type PersistentVolumeClaims interface {
	New(name types.NamespacedName, owner meta.Object) PersistentVolumeClaim
}

func New(client client.Client, scheme *runtime.Scheme) PersistentVolumeClaims {
	return &claims{client: client, scheme: scheme}
}

type claims struct {
	client client.Client
	scheme *runtime.Scheme
}

func (c *claims) New(name types.NamespacedName, owner meta.Object) PersistentVolumeClaim {
	return &claim{
		client: c.client,
		scheme: c.scheme,
		name:   name,
		owner:  owner,
		size:   resource.MustParse("5Gi"),
	}
}

type claim struct {
	client             client.Client
	scheme             *runtime.Scheme
	name               types.NamespacedName
	owner              meta.Object
	path               string
	size               resource.Quantity
	volumeNodeAffinity *core.VolumeNodeAffinity
}

func (c *claim) SetStoragePath(path string) {
	c.path = path
}

func (c *claim) SetStorageSize(quantity resource.Quantity) {
	c.size = quantity
}

func (c *claim) SetNodeSelector(nodeSelectors map[string]string) {
	nodeSelectorMatchExpressions := []core.NodeSelectorRequirement{}
	for k, v := range nodeSelectors {
		valueList := []string{v}
		expression := core.NodeSelectorRequirement{
			Key:      k,
			Operator: core.NodeSelectorOpIn,
			Values:   valueList,
		}
		nodeSelectorMatchExpressions = append(nodeSelectorMatchExpressions, expression)
	}
	nodeSelectorTerm := core.NodeSelector{
		NodeSelectorTerms: []core.NodeSelectorTerm{{
			MatchExpressions: nodeSelectorMatchExpressions,
		}},
	}
	c.volumeNodeAffinity = &core.VolumeNodeAffinity{
		Required: &nodeSelectorTerm,
	}

}

func (c *claim) EnsureExists() error {
	if c.path != "" {
		volumeMode := core.PersistentVolumeFilesystem
		hostPathType := core.HostPathDirectoryOrCreate
		pv := &core.PersistentVolume{
			ObjectMeta: meta.ObjectMeta{
				Name:      c.name.Name + "-pv",
				Namespace: c.name.Namespace,
				Labels: map[string]string{
					"claim-name": c.name.Name,
				},
			},
			Spec: core.PersistentVolumeSpec{
				Capacity: map[core.ResourceName]resource.Quantity{
					core.ResourceStorage: c.size,
				},
				VolumeMode: &volumeMode,
				AccessModes: []core.PersistentVolumeAccessMode{
					core.ReadWriteOnce,
				},
				PersistentVolumeReclaimPolicy: core.PersistentVolumeReclaimDelete,
				NodeAffinity:                  c.volumeNodeAffinity,
				PersistentVolumeSource: core.PersistentVolumeSource{
					// TODO: HostPath has to be changed to Local persistent volume with multi-node approach.
					// TODO: We should create init container with hostPath volume and mount local pv to the same path.
					HostPath: &core.HostPathVolumeSource{
						Path: c.path,
						Type: &hostPathType,
					},
				},
			},
		}
		if err := c.client.Create(context.Background(), pv); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

	}
	pvc := &core.PersistentVolumeClaim{
		ObjectMeta: meta.ObjectMeta{
			Name:      c.name.Name,
			Namespace: c.name.Namespace,
		},
	}

	pvc.Spec = core.PersistentVolumeClaimSpec{
		AccessModes: []core.PersistentVolumeAccessMode{core.ReadWriteOnce},
		Resources: core.ResourceRequirements{
			Requests: map[core.ResourceName]resource.Quantity{
				core.ResourceStorage: c.size,
			},
		},
	}

	if c.path != "" {
		storageClassName := ""
		pvc.Spec.StorageClassName = &storageClassName

		if pvc.Spec.Selector == nil {
			pvc.Spec.Selector = &meta.LabelSelector{
				MatchLabels: map[string]string{},
			}
		}

		pvc.Spec.Selector.MatchLabels["claim-name"] = c.name.Name
	}

	_, err := controllerutil.CreateOrUpdate(context.Background(), c.client, pvc, func() error {
		return controllerutil.SetControllerReference(c.owner, pvc, c.scheme)
	})

	return err
}
