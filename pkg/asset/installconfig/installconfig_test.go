package installconfig

import (
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	netopv1 "github.com/openshift/cluster-network-operator/pkg/apis/networkoperator/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/asset/mock"
	"github.com/openshift/installer/pkg/ipnet"
	"github.com/openshift/installer/pkg/types"
	"github.com/openshift/installer/pkg/types/aws"
	"github.com/openshift/installer/pkg/types/none"
)

func validInstallConfig() *types.InstallConfig {
	return &types.InstallConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		BaseDomain: "test-domain",
		Platform: types.Platform{
			AWS: &aws.Platform{
				Region: "us-east-1",
			},
		},
		PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
	}
}

func TestInstallConfigGenerate_FillsInDefaults(t *testing.T) {
	sshPublicKey := &sshPublicKey{}
	baseDomain := &baseDomain{"test-domain"}
	clusterName := &clusterName{"test-cluster"}
	pullSecret := &pullSecret{`{"auths":{"example.com":{"auth":"authorization value"}}}`}
	platform := &platform{
		None: &none.Platform{},
	}
	installConfig := &InstallConfig{}
	parents := asset.Parents{}
	parents.Add(
		sshPublicKey,
		baseDomain,
		clusterName,
		pullSecret,
		platform,
	)
	if err := installConfig.Generate(parents); err != nil {
		t.Errorf("unexpected error generating install config: %v", err)
	}
	expected := &types.InstallConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster",
		},
		BaseDomain: "test-domain",
		Networking: &types.Networking{
			MachineCIDR: ipnet.MustParseCIDR("10.0.0.0/16"),
			Type:        "OpenshiftSDN",
			ServiceCIDR: ipnet.MustParseCIDR("172.30.0.0/16"),
			ClusterNetworks: []netopv1.ClusterNetwork{
				{
					CIDR:             "10.128.0.0/14",
					HostSubnetLength: 9,
				},
			},
		},
		Machines: []types.MachinePool{
			{
				Name:     "master",
				Replicas: func(x int64) *int64 { return &x }(3),
			},
			{
				Name:     "worker",
				Replicas: func(x int64) *int64 { return &x }(3),
			},
		},
		Platform: types.Platform{
			None: &none.Platform{},
		},
		PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
	}
	assert.Equal(t, expected, installConfig.Config, "unexpected config generated")
}

func TestInstallConfigLoad(t *testing.T) {
	cases := []struct {
		name           string
		data           string
		fetchError     error
		expectedFound  bool
		expectedError  bool
		expectedConfig *types.InstallConfig
	}{
		{
			name: "valid InstallConfig",
			data: `
apiVersion: v1beta1
metadata:
  name: test-cluster
baseDomain: test-domain
platform:
  aws:
    region: us-east-1
pullSecret: "{\"auths\":{\"example.com\":{\"auth\":\"authorization value\"}}}"
`,
			expectedFound: true,
			expectedConfig: &types.InstallConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
				},
				BaseDomain: "test-domain",
				Networking: &types.Networking{
					MachineCIDR: ipnet.MustParseCIDR("10.0.0.0/16"),
					Type:        "OpenshiftSDN",
					ServiceCIDR: ipnet.MustParseCIDR("172.30.0.0/16"),
					ClusterNetworks: []netopv1.ClusterNetwork{
						{
							CIDR:             "10.128.0.0/14",
							HostSubnetLength: 9,
						},
					},
				},
				Machines: []types.MachinePool{
					{
						Name:     "master",
						Replicas: func(x int64) *int64 { return &x }(3),
					},
					{
						Name:     "worker",
						Replicas: func(x int64) *int64 { return &x }(3),
					},
				},
				Platform: types.Platform{
					AWS: &aws.Platform{
						Region: "us-east-1",
					},
				},
				PullSecret: `{"auths":{"example.com":{"auth":"authorization value"}}}`,
			},
		},
		{
			name: "invalid InstallConfig",
			data: `
metadata:
  name: test-cluster
`,
			expectedError: true,
		},
		{
			name:          "empty",
			data:          "",
			expectedError: true,
		},
		{
			name:          "not yaml",
			data:          "This is not yaml.",
			expectedError: true,
		},
		{
			name:       "file not found",
			fetchError: &os.PathError{Err: os.ErrNotExist},
		},
		{
			name:          "error fetching file",
			fetchError:    errors.New("fetch failed"),
			expectedError: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			fileFetcher := mock.NewMockFileFetcher(mockCtrl)
			fileFetcher.EXPECT().FetchByName(installConfigFilename).
				Return(
					&asset.File{
						Filename: installConfigFilename,
						Data:     []byte(tc.data)},
					tc.fetchError,
				)

			ic := &InstallConfig{}
			found, err := ic.Load(fileFetcher)
			assert.Equal(t, tc.expectedFound, found, "unexpected found value returned from Load")
			if tc.expectedError {
				assert.Error(t, err, "expected error from Load")
			} else {
				assert.NoError(t, err, "unexpected error from Load")
			}
			if tc.expectedFound {
				assert.Equal(t, tc.expectedConfig, ic.Config, "unexpected Config in InstallConfig")
			}
		})
	}
}
