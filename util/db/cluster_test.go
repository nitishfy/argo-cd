package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	fakeNamespace = "fake-ns"
)

func Test_URIToSecretName(t *testing.T) {
	name, err := URIToSecretName("cluster", "http://foo")
	require.NoError(t, err)
	assert.Equal(t, "cluster-foo-752281925", name)

	name, err = URIToSecretName("cluster", "http://thelongestdomainnameintheworld.argocd-project.com:3000")
	require.NoError(t, err)
	assert.Equal(t, "cluster-thelongestdomainnameintheworld.argocd-project.com-2721640553", name)

	name, err = URIToSecretName("cluster", "http://[fe80::1ff:fe23:4567:890a]")
	require.NoError(t, err)
	assert.Equal(t, "cluster-fe80--1ff-fe23-4567-890a-3877258831", name)

	name, err = URIToSecretName("cluster", "http://[fe80::1ff:fe23:4567:890a]:8000")
	require.NoError(t, err)
	assert.Equal(t, "cluster-fe80--1ff-fe23-4567-890a-664858999", name)

	name, err = URIToSecretName("cluster", "http://[FE80::1FF:FE23:4567:890A]:8000")
	require.NoError(t, err)
	assert.Equal(t, "cluster-fe80--1ff-fe23-4567-890a-682802007", name)

	name, err = URIToSecretName("cluster", "http://:/abc")
	require.NoError(t, err)
	assert.Equal(t, "cluster--1969338796", name)
}

func Test_secretToCluster(t *testing.T) {
	labels := map[string]string{"key1": "val1"}
	annotations := map[string]string{"key2": "val2"}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mycluster",
			Namespace:   fakeNamespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
			"config": []byte("{\"username\":\"foo\"}"),
		},
	}
	cluster, err := SecretToCluster(secret)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.Cluster{
		Name:   "test",
		Server: "http://mycluster",
		Config: v1alpha1.ClusterConfig{
			Username: "foo",
		},
		Labels:      labels,
		Annotations: annotations,
	}, *cluster)
}

func Test_secretToCluster_LastAppliedConfigurationDropped(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mycluster",
			Namespace:   fakeNamespace,
			Annotations: map[string]string{corev1.LastAppliedConfigAnnotation: "val2"},
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
			"config": []byte("{\"username\":\"foo\"}"),
		},
	}
	cluster, err := SecretToCluster(secret)
	require.NoError(t, err)
	assert.Empty(t, cluster.Annotations)
}

func TestClusterToSecret(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		Server:      "server",
		Labels:      map[string]string{"test": "label"},
		Annotations: map[string]string{"test": "annotation"},
		Name:        "test",
		Config:      v1alpha1.ClusterConfig{},
		Project:     "project",
		Namespaces:  []string{"default"},
	}
	s := &corev1.Secret{}
	err := clusterToSecret(cluster, s)
	require.NoError(t, err)

	assert.Equal(t, []byte(cluster.Server), s.Data["server"])
	assert.Equal(t, []byte(cluster.Name), s.Data["name"])
	assert.Equal(t, []byte(cluster.Project), s.Data["project"])
	assert.Equal(t, []byte("default"), s.Data["namespaces"])
	assert.Equal(t, cluster.Annotations, s.Annotations)
	assert.Equal(t, cluster.Labels, s.Labels)
}

func TestClusterToSecret_LastAppliedConfigurationRejected(t *testing.T) {
	cluster := &v1alpha1.Cluster{
		Server:      "server",
		Annotations: map[string]string{corev1.LastAppliedConfigAnnotation: "val2"},
		Name:        "test",
		Config:      v1alpha1.ClusterConfig{},
		Project:     "project",
		Namespaces:  []string{"default"},
	}
	s := &corev1.Secret{}
	err := clusterToSecret(cluster, s)
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func Test_secretToCluster_NoConfig(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
		},
	}
	cluster, err := SecretToCluster(secret)
	require.NoError(t, err)
	assert.Equal(t, v1alpha1.Cluster{
		Name:        "test",
		Server:      "http://mycluster",
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}, *cluster)
}

func Test_secretToCluster_InvalidConfig(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
			"config": []byte("{'tlsClientConfig':{'insecure':false}}"),
		},
	}
	cluster, err := SecretToCluster(secret)
	require.Error(t, err)
	assert.Nil(t, cluster)
}

func TestUpdateCluster(t *testing.T) {
	kubeclientset := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("http://mycluster"),
			"config": []byte("{}"),
		},
	})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	requestedAt := metav1.Now()
	_, err := db.UpdateCluster(context.Background(), &v1alpha1.Cluster{
		Name:               "test",
		Server:             "http://mycluster",
		RefreshRequestedAt: &requestedAt,
	})
	require.NoError(t, err)

	secret, err := kubeclientset.CoreV1().Secrets(fakeNamespace).Get(context.Background(), "mycluster", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, secret.Annotations[v1alpha1.AnnotationKeyRefresh], requestedAt.Format(time.RFC3339))
}

func TestDeleteUnknownCluster(t *testing.T) {
	kubeclientset := fake.NewClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("http://mycluster"),
			"name":   []byte("mycluster"),
		},
	})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	assert.EqualError(t, db.DeleteCluster(context.Background(), "http://unknown"), `rpc error: code = NotFound desc = cluster "http://unknown" not found`)
}

func TestRejectCreationForInClusterWhenDisabled(t *testing.T) {
	argoCDConfigMapWithInClusterServerAddressDisabled := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{"cluster.inClusterEnabled": "false"},
	}
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}
	kubeclientset := fake.NewClientset(argoCDConfigMapWithInClusterServerAddressDisabled, argoCDSecret)
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
		Server: v1alpha1.KubernetesInternalAPIServerAddr,
		Name:   "incluster-name",
	})
	require.Error(t, err)
}

func runWatchTest(t *testing.T, db ArgoDB, actions []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster)) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	timeout := time.Second * 5

	allDone := make(chan bool, 1)

	doNext := func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
		if len(actions) == 0 {
			assert.Fail(t, "Unexpected event")
		}
		next := actions[0]
		next(old, new)
		if t.Failed() {
			allDone <- true
		}
		if len(actions) == 1 {
			allDone <- true
		} else {
			actions = actions[1:]
		}
	}

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			doNext(nil, cluster)
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			doNext(oldCluster, newCluster)
		}, func(clusterServer string) {
			doNext(&v1alpha1.Cluster{Server: clusterServer}, nil)
		}))
	}()

	select {
	case <-allDone:
	case <-time.After(timeout):
		assert.Fail(t, "Failed due to timeout")
	}
}

func TestListClusters(t *testing.T) {
	emptyArgoCDConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDConfigMapWithInClusterServerAddressDisabled := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{"cluster.inClusterEnabled": "false"},
	}
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}
	secretForServerWithInClusterAddr := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster1",
			Namespace: fakeNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte(v1alpha1.KubernetesInternalAPIServerAddr),
			"name":   []byte("in-cluster"),
		},
	}

	secretForServerWithExternalClusterAddr := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster2",
			Namespace: fakeNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("http://mycluster2"),
			"name":   []byte("mycluster2"),
		},
	}

	invalidSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster3",
			Namespace: fakeNamespace,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster3"),
			"config": []byte("{'tlsClientConfig':{'insecure':false}}"),
		},
	}

	t.Run("Valid clusters", func(t *testing.T) {
		kubeclientset := fake.NewClientset(secretForServerWithInClusterAddr, secretForServerWithExternalClusterAddr, emptyArgoCDConfigMap, argoCDSecret)
		settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
		db := NewDB(fakeNamespace, settingsManager, kubeclientset)

		clusters, err := db.ListClusters(context.TODO())
		require.NoError(t, err)
		assert.Len(t, clusters.Items, 2)
	})

	t.Run("Cluster list with invalid cluster", func(t *testing.T) {
		kubeclientset := fake.NewClientset(secretForServerWithInClusterAddr, secretForServerWithExternalClusterAddr, invalidSecret, emptyArgoCDConfigMap, argoCDSecret)
		settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
		db := NewDB(fakeNamespace, settingsManager, kubeclientset)

		clusters, err := db.ListClusters(context.TODO())
		require.NoError(t, err)
		assert.Len(t, clusters.Items, 2)
	})

	t.Run("Implicit in-cluster secret", func(t *testing.T) {
		kubeclientset := fake.NewClientset(secretForServerWithExternalClusterAddr, emptyArgoCDConfigMap, argoCDSecret)
		settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
		db := NewDB(fakeNamespace, settingsManager, kubeclientset)

		clusters, err := db.ListClusters(context.TODO())
		require.NoError(t, err)
		// ListClusters() should have added an implicit in-cluster secret to the list
		assert.Len(t, clusters.Items, 2)
	})

	t.Run("ListClusters() should not add the cluster with in-cluster server address since in-cluster is disabled", func(t *testing.T) {
		kubeclientset := fake.NewClientset(secretForServerWithInClusterAddr, argoCDConfigMapWithInClusterServerAddressDisabled, argoCDSecret)
		settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
		db := NewDB(fakeNamespace, settingsManager, kubeclientset)

		clusters, err := db.ListClusters(context.TODO())
		require.NoError(t, err)
		assert.Empty(t, clusters.Items)
	})

	t.Run("ListClusters() should add this cluster since it does not contain in-cluster server address even though in-cluster is disabled", func(t *testing.T) {
		kubeclientset := fake.NewClientset(secretForServerWithExternalClusterAddr, argoCDConfigMapWithInClusterServerAddressDisabled, argoCDSecret)
		settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
		db := NewDB(fakeNamespace, settingsManager, kubeclientset)

		clusters, err := db.ListClusters(context.TODO())
		require.NoError(t, err)
		assert.Len(t, clusters.Items, 1)
	})
}

// TestClusterRaceConditionClusterSecrets reproduces a race condition
// on the cluster secrets. The test isn't asserting anything because
// before the fix it would cause a panic from concurrent map iteration and map write
func TestClusterRaceConditionClusterSecrets(_ *testing.T) {
	clusterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("http://mycluster"),
			"config": []byte("{}"),
		},
	}
	kubeClient := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
		clusterSecret,
	)
	settingsManager := settings.NewSettingsManager(context.Background(), kubeClient, "default")
	db := NewDB("default", settingsManager, kubeClient)
	cluster, _ := SecretToCluster(clusterSecret)
	go func() {
		for {
			// create a copy so we dont act on the same argo cluster
			clusterCopy := cluster.DeepCopy()
			_, _ = db.UpdateCluster(context.Background(), clusterCopy)
		}
	}()
	// yes, we will take 15 seconds to run this test
	// but it reliably triggered the race condition
	for i := 0; i < 30; i++ {
		// create a copy so we dont act on the same argo cluster
		clusterCopy := cluster.DeepCopy()
		_, _ = db.UpdateCluster(context.Background(), clusterCopy)
		time.Sleep(time.Millisecond * 500)
	}
}
