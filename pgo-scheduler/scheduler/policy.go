package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	crv1 "github.com/crunchydata/postgres-operator/apis/cr/v1"
	"github.com/crunchydata/postgres-operator/apiserver"
	"github.com/crunchydata/postgres-operator/kubeapi"
	v1batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyJob struct {
	ccpImageTag    string
	ccpImagePrefix string
	cluster        string
	namespace      string
	secret         string
	policy         string
	database       string
}

func (s *ScheduleTemplate) NewPolicySchedule() PolicyJob {
	return PolicyJob{
		namespace:      s.Namespace,
		cluster:        s.Cluster,
		ccpImageTag:    s.Policy.ImageTag,
		ccpImagePrefix: s.Policy.ImagePrefix,
		secret:         s.Policy.Secret,
		policy:         s.Policy.Name,
		database:       s.Policy.Database,
	}
}

func (p PolicyJob) Run() {
	contextLogger := log.WithFields(log.Fields{
		"namespace": p.namespace,
		"policy":    p.policy,
		"cluster":   p.cluster})

	contextLogger.Info("Running Policy schedule")

	cluster := crv1.Pgcluster{}
	found, err := kubeapi.Getpgcluster(restClient, &cluster, p.cluster, p.namespace)
	if !found {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("pgCluster not found")
		return
	} else if err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("error retrieving pgCluster")
		return
	}

	policy := crv1.Pgpolicy{}
	found, err = kubeapi.Getpgpolicy(restClient, &policy, p.policy, p.namespace)
	if !found {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("pgPolicy not found")
		return
	} else if err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("error retrieving pgPolicy")
		return
	}

	name := fmt.Sprintf("policy-%s-%s-schedule", p.cluster, p.policy)

	filename := fmt.Sprintf("%s.sql", p.policy)
	data := make(map[string]string)
	data[filename] = string(policy.Spec.SQL)

	var labels = map[string]string{
		"pg-cluster": p.cluster,
	}
	labels["pg-cluster"] = p.cluster
	labels["pg-policy"] = p.policy
	labels["pg-schedule"] = "true"

	configmap := &v1.ConfigMap{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Data: data,
	}

	err = kubeapi.DeleteConfigMap(apiserver.Clientset, name, p.namespace)
	if err != nil && !kerrors.IsNotFound(err) {
		contextLogger.WithFields(log.Fields{
			"error":     err,
			"configMap": name,
		}).Error("could not delete policy configmap")
		return
	}

	log.Debug("Creating configmap..")
	err = kubeapi.CreateConfigMap(apiserver.Clientset, configmap, p.namespace)
	if err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("could not create policy configmap")
		return
	}

	policyJob := PolicyTemplate{
		JobName:        name,
		ClusterName:    p.cluster,
		COImagePrefix:  p.ccpImagePrefix,
		COImageTag:     p.ccpImageTag,
		PGHost:         p.cluster,
		PGPort:         cluster.Spec.Port,
		PGDatabase:     p.database,
		PGSQLConfigMap: name,
		PGUserSecret:   p.secret,
	}

	var doc bytes.Buffer
	if err := PolicyJobTemplate.Execute(&doc, policyJob); err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err}).Error("Failed to render job template")
		return
	}

	oldJob, found := kubeapi.GetJob(kubeClient, name, p.namespace)
	if found {
		err = kubeapi.DeleteJob(kubeClient, name, p.namespace)
		if err != nil {
			contextLogger.WithFields(log.Fields{
				"job":   name,
				"error": err,
			}).Error("error deleting policy job")
			return
		}

		timeout := time.Second * 60
		err = kubeapi.IsJobDeleted(kubeClient, p.namespace, oldJob, timeout)
		if err != nil {
			contextLogger.WithFields(log.Fields{
				"job":   name,
				"error": err,
			}).Error("error waiting for job to delete")
			return
		}
	}

	newJob := &v1batch.Job{}
	if err := json.Unmarshal(doc.Bytes(), newJob); err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("Failed unmarshaling job template")
		return
	}

	_, err = kubeapi.CreateJob(kubeClient, newJob, p.namespace)
	if err != nil {
		contextLogger.WithFields(log.Fields{
			"error": err,
		}).Error("Failed creating policy job")
		return
	}
}
