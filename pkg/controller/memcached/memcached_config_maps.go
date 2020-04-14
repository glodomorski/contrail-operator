package memcached

import (
	"bytes"
	"text/template"

	core "k8s.io/api/core/v1"

	contrail "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/Juniper/contrail-operator/pkg/k8s"
)

type configMaps struct {
	cm            *k8s.ConfigMap
	memcachedSpec contrail.MemcachedSpec
}

func (r *ReconcileMemcached) configMap(configMapName string, memcached *contrail.Memcached) *configMaps {
	return &configMaps{
		cm:            r.kubernetes.ConfigMap(configMapName, "Memcached", memcached),
		memcachedSpec: memcached.Spec,
	}
}

func (c *configMaps) ensureExists(podIP string) error {
	spc := &memcachedConfig{
		ListenPort:      c.memcachedSpec.ServiceConfiguration.GetListenPort(),
		ConnectionLimit: c.memcachedSpec.ServiceConfiguration.GetConnectionLimit(),
		MaxMemory:       c.memcachedSpec.ServiceConfiguration.GetMaxMemory(),
		PodIP:           podIP,
	}
	return c.cm.EnsureExists(spc)
}

type memcachedConfig struct {
	ListenPort      int32
	ConnectionLimit int32
	MaxMemory       int32
	PodIP           string
}

func (c *memcachedConfig) FillConfigMap(cm *core.ConfigMap) {
	cm.Data["config.json"] = c.String()
}

func (c *memcachedConfig) String() string {
	log.Info("TEST ", "POD IP ", c.PodIP)
	memcachedConfig := template.Must(template.New("").Parse(memcachedConfigTemplate))
	var buffer bytes.Buffer
	if err := memcachedConfig.Execute(&buffer, c); err != nil {
		panic(err)
	}
	return buffer.String()
}

// /usr/bin/memcached -v -l 0.0.0.0 -p 11211 -c 5000 -U 0 -m 256 -Z -o ssl_chain_cert="/etc/certificates/server-172.18.0.3.crt" ssl_key="/etc/certificates/server-key-172.18.0.3.pem"
const memcachedConfigTemplate = `{
	"command": "/usr/bin/memcached -v -l 0.0.0.0 -p {{ .ListenPort }} -c {{ .ConnectionLimit }} -U 0 -m {{ .MaxMemory }}",
	"config_files": []
}`
