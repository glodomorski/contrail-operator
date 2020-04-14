package memcached

import (
	"context"
	"fmt"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	contrail "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/k8s"
)

var log = logf.Log.WithName("controller_memcached")

// Add creates a new Memcached Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	config := mgr.GetConfig()
	return NewReconcileMemcached(mgr.GetClient(), mgr.GetScheme(), k8s.New(mgr.GetClient(), mgr.GetScheme()), config)
}

func NewReconcileMemcached(client client.Client, scheme *runtime.Scheme, kubernetes *k8s.Kubernetes, config *rest.Config) *ReconcileMemcached {
	return &ReconcileMemcached{
		client:     client,
		scheme:     scheme,
		kubernetes: kubernetes,
		config:     config,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	c, err := controller.New("memcached-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &contrail.Memcached{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &apps.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &contrail.Memcached{},
	})
	return err
}

// blank assignment to verify that ReconcileMemcached implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMemcached{}

// ReconcileMemcached reconciles a Memcached object
type ReconcileMemcached struct {
	client     client.Client
	scheme     *runtime.Scheme
	kubernetes *k8s.Kubernetes
	config     *rest.Config
}

// Reconcile reads that state of the cluster for a Memcached object and makes changes based on the state read
// and what is in the Memcached.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMemcached) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Memcached")
	memcachedCR := &contrail.Memcached{}
	err := r.client.Get(context.TODO(), request.NamespacedName, memcachedCR)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !memcachedCR.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	memcachedPods, err := r.listMemcachedPods(memcachedCR.Name)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to list memcached pods: %v", err)
	}
	log.Info("TEST :listed memcached pods")
	if err := r.ensureCertificatesExist(memcachedCR, memcachedPods); err != nil {
		return reconcile.Result{}, err
	}
	if len(memcachedPods.Items) > 0 {
		log.Info("TEST : len of memecached pods > 0")
		err = contrail.SetPodsToReady(memcachedPods, r.client)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	ips := memcachedCR.Status.IPs
	if len(ips) == 0 {
		ips = []string{"0.0.0.0"}
	}
	memcachedConfigMapName := memcachedCR.Name + "-config"
	if err = r.configMap(memcachedConfigMapName, memcachedCR).ensureExists(ips[0]); err != nil {
		return reconcile.Result{}, err
	}
	deployment := &apps.Deployment{
		ObjectMeta: meta.ObjectMeta{
			Namespace: request.Namespace,
			Name:      request.Name + "-deployment",
		},
	}
	_, err = controllerutil.CreateOrUpdate(context.Background(), r.client, deployment, func() error {
		labels := map[string]string{"Memcached": request.Name}
		deployment.Spec.Template.ObjectMeta.Labels = labels
		deployment.ObjectMeta.Labels = labels
		deployment.Spec.Selector = &meta.LabelSelector{MatchLabels: labels}
		updateMemcachedPodSpec(&deployment.Spec.Template.Spec, memcachedCR, memcachedConfigMapName)
		return controllerutil.SetControllerReference(memcachedCR, deployment, r.scheme)
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, r.updateStatus(memcachedCR, deployment)
}

func (r *ReconcileMemcached) updateStatus(memcachedCR *contrail.Memcached, deployment *apps.Deployment) error {
	err := r.client.Get(context.Background(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, deployment)
	if err != nil {
		return err
	}
	expectedReplicas := int32(1)
	if deployment.Spec.Replicas != nil {
		expectedReplicas = *deployment.Spec.Replicas
	}
	memcachedCR.Status.IPs = []string{}
	if deployment.Status.ReadyReplicas == expectedReplicas {
		pods := core.PodList{}
		var labels client.MatchingLabels = deployment.Spec.Selector.MatchLabels
		if err = r.client.List(context.Background(), &pods, labels); err != nil {
			return err
		}
		if len(pods.Items) < 1 {
			return fmt.Errorf("ReconcileMemchached.updateStatus: expected atleast 1 pod with labels %v, got %d", labels, len(pods.Items))
		}
		podIP := "127.0.0.1"
		for _, pod := range pods.Items {
			if pod.Status.PodIP != "" {
				memcachedCR.Status.IPs = append(memcachedCR.Status.IPs, pod.Status.PodIP)
				podIP = pod.Status.PodIP
			}
		}
		port := memcachedCR.Spec.ServiceConfiguration.GetListenPort()
		memcachedCR.Status.Node = fmt.Sprintf("%s:%d", podIP, port)
		memcachedCR.Status.Active = true
	} else {
		memcachedCR.Status.Active = false
	}
	return r.client.Status().Update(context.Background(), memcachedCR)
}

func updateMemcachedPodSpec(podSpec *core.PodSpec, memcachedCR *contrail.Memcached, configMapName string) {
	podSpec.HostNetwork = true
	podSpec.Tolerations = []core.Toleration{
		{
			Operator: core.TolerationOpExists,
			Effect:   core.TaintEffectNoSchedule,
		},
		{
			Operator: core.TolerationOpExists,
			Effect:   core.TaintEffectNoExecute,
		},
	}
	volumes := []core.Volume{
		{
			Name: "config-volume",
			VolumeSource: core.VolumeSource{
				ConfigMap: &core.ConfigMapVolumeSource{
					LocalObjectReference: core.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		},
	}
	volumes = append(volumes, core.Volume{
		Name: memcachedCR.Name + "-secret-certificates",
		VolumeSource: core.VolumeSource{
			Secret: &core.SecretVolumeSource{
				SecretName: memcachedCR.Name + "-secret-certificates",
			},
		},
	})
	defMode := int32(420)
	volumes = append(volumes, core.Volume{
		Name: "status",
		VolumeSource: core.VolumeSource{
			DownwardAPI: &core.DownwardAPIVolumeSource{
				Items: []core.DownwardAPIVolumeFile{
					{
						FieldRef: &core.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.labels",
						},
						Path: "pod_labels",
					},
				},
				DefaultMode: &defMode,
			},
		},
	})
	podSpec.Volumes = volumes
	podSpec.InitContainers = []core.Container{
		{
			Name:            "wait-for-ready-conf",
			ImagePullPolicy: core.PullAlways,
			Image:           getImage(memcachedCR, "waitForReadyConf"),
			Command:         []string{"sh", "-c", "until grep ready /tmp/podinfo/pod_labels > /dev/null 2>&1; do sleep 1; done"},
			VolumeMounts: []core.VolumeMount{{
				Name:      "status",
				MountPath: "/tmp/podinfo",
			}},
		},
	}
	podSpec.Containers = []core.Container{memcachedContainer(memcachedCR)}
}

func memcachedContainer(memcachedCR *contrail.Memcached) core.Container {
	return core.Container{
		Name:            "memcached",
		Image:           getImage(memcachedCR, "memcached"),
		ImagePullPolicy: core.PullAlways,
		Env: []core.EnvVar{{
			Name:  "KOLLA_SERVICE_NAME",
			Value: "memcached",
		}, {
			Name:  "KOLLA_CONFIG_STRATEGY",
			Value: "COPY_ALWAYS",
		}},
		Ports: []core.ContainerPort{{
			ContainerPort: memcachedCR.Spec.ServiceConfiguration.GetListenPort(),
			Name:          "memcached",
		}},
		VolumeMounts: []core.VolumeMount{
			{
				Name:      "config-volume",
				ReadOnly:  true,
				MountPath: "/var/lib/kolla/config_files/",
			},
			{
				Name:      memcachedCR.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			},
		},
	}
}

func (r *ReconcileMemcached) ensureCertificatesExist(memcached *contrail.Memcached, pods *core.PodList) error {
	hostNetwork := true
	if memcached.Spec.ServiceConfiguration.HostNetwork != nil {
		hostNetwork = *memcached.Spec.ServiceConfiguration.HostNetwork
	}
	log.Info("TEST :CReting secret")
	return certificates.New(r.client, r.scheme, memcached, r.config, pods, "Memcached", hostNetwork).EnsureExistsAndIsSigned()
}

func (r *ReconcileMemcached) listMemcachedPods(memcachedName string) (*core.PodList, error) {
	pods := &core.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"Memcached": memcachedName})
	listOpts := client.ListOptions{LabelSelector: labelSelector}
	if err := r.client.List(context.TODO(), pods, &listOpts); err != nil {
		return &core.PodList{}, err
	}
	return pods, nil
}

func getImage(cr *contrail.Memcached, containerName string) string {
	var defaultContainersImages = map[string]string{
		"waitForReadyConf": "localhost:5000/busybox",
		"memcached":        "localhost:5000/centos-binary-memcached:train",
	}

	c, ok := cr.Spec.ServiceConfiguration.Containers[containerName]
	if !ok || c == nil {
		return defaultContainersImages[containerName]
	}

	return c.Image
}
