package command

import (
	corev1 "k8s.io/api/core/v1"

	contrail "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/k8s"
)

type configMaps struct {
	cm                      *k8s.ConfigMap
	ccSpec                  contrail.CommandSpec
	keystoneAdminPassSecret *corev1.Secret
	swiftCredentialsSecret  *corev1.Secret
}

func (r *ReconcileCommand) configMap(
	configMapName string, ownerType string, cc *contrail.Command, keystoneSecret *corev1.Secret, swiftSecret *corev1.Secret,
) *configMaps {
	return &configMaps{
		cm:                      r.kubernetes.ConfigMap(configMapName, ownerType, cc),
		ccSpec:                  cc.Spec,
		keystoneAdminPassSecret: keystoneSecret,
		swiftCredentialsSecret:  swiftSecret,
	}
}

func (c *configMaps) ensureCommandConfigExist(hostIP string, keystoneAddress string, keystonePort int, keystoneAuthProtocol, postgresAddress, ConfigApiEndpoint, AnalyticsEndpoint string) error {
	cc := &commandConf{
		ClusterName:          "default",
		AdminUsername:        "admin",
		AdminPassword:        string(c.keystoneAdminPassSecret.Data["password"]),
		SwiftUsername:        string(c.swiftCredentialsSecret.Data["user"]),
		SwiftPassword:        string(c.swiftCredentialsSecret.Data["password"]),
		ConfigAPIURL:         ConfigApiEndpoint,
		TelemetryURL:         AnalyticsEndpoint,
		PostgresAddress:      postgresAddress,
		PostgresUser:         "root",
		PostgresDBName:       "contrail_test",
		HostIP:               hostIP,
		CAFilePath:           certificates.SignerCAFilepath,
		PGPassword:           "contrail123",
		KeystoneAddress:      keystoneAddress,
		KeystonePort:         keystonePort,
		KeystoneAuthProtocol: keystoneAuthProtocol,
		ContrailVersion:      c.ccSpec.ServiceConfiguration.ContrailVersion,
	}

	if c.ccSpec.ServiceConfiguration.ClusterName != "" {
		cc.ClusterName = c.ccSpec.ServiceConfiguration.ClusterName
	}
	return c.cm.EnsureExists(cc)
}
