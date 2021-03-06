package scheduleservice

/*
 Copyright 2017-2019 Crunchy Data Solutions, Inc.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	crv1 "github.com/crunchydata/postgres-operator/apis/cr/v1"
	"github.com/crunchydata/postgres-operator/apiserver"
	msgs "github.com/crunchydata/postgres-operator/apiservermsgs"
	"github.com/crunchydata/postgres-operator/kubeapi"
	"github.com/crunchydata/postgres-operator/util"

	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type scheduleRequest struct {
	Request  *msgs.CreateScheduleRequest
	Response *msgs.CreateScheduleResponse
}

func (s scheduleRequest) createBackRestSchedule(cluster *crv1.Pgcluster) *PgScheduleSpec {
	name := fmt.Sprintf("%s-%s-%s", cluster.Name, s.Request.ScheduleType, s.Request.PGBackRestType)
	schedule := &PgScheduleSpec{
		Name:      name,
		Cluster:   cluster.Name,
		Version:   "v1",
		Created:   time.Now().Format(time.RFC3339),
		Schedule:  s.Request.Schedule,
		Type:      s.Request.ScheduleType,
		Namespace: apiserver.Namespace,
		PGBackRest: PGBackRest{
			Label:     fmt.Sprintf("pg-cluster=%s,service-name=%s", cluster.Name, cluster.Name),
			Container: "database",
			Type:      s.Request.PGBackRestType,
		},
	}
	return schedule
}

func (s scheduleRequest) createBaseBackupSchedule(cluster *crv1.Pgcluster) *PgScheduleSpec {
	name := fmt.Sprintf("backup-%s", cluster.Name)

	if s.Request.PVCName != "" {
		_, exists, err := kubeapi.GetPVC(apiserver.Clientset, s.Request.PVCName, apiserver.Namespace)
		if err != nil {
			s.Response.Status.Code = msgs.Error
			s.Response.Status.Msg = err.Error()
			return &PgScheduleSpec{}
		} else if !exists {
			s.Response.Status.Code = msgs.Error
			s.Response.Status.Msg = fmt.Sprintf("PVC does not exist for backup: %s", s.Request.PVCName)
			return &PgScheduleSpec{}
		}
	}

	schedule := &PgScheduleSpec{
		Name:      name,
		Cluster:   cluster.Name,
		Version:   "v1",
		Created:   time.Now().Format(time.RFC3339),
		Schedule:  s.Request.Schedule,
		Type:      s.Request.ScheduleType,
		Namespace: apiserver.Namespace,
		PGBaseBackup: PGBaseBackup{
			BackupVolume: s.Request.PVCName,
			ImagePrefix:  apiserver.Pgo.Cluster.CCPImagePrefix,
			ImageTag:     apiserver.Pgo.Cluster.CCPImageTag,
			Secret:       cluster.Spec.PrimarySecretName,
		},
	}
	return schedule
}

func (s scheduleRequest) createPolicySchedule(cluster *crv1.Pgcluster) *PgScheduleSpec {
	name := fmt.Sprintf("%s-%s-%s", cluster.Name, s.Request.ScheduleType, s.Request.PolicyName)

	err := util.ValidatePolicy(apiserver.RESTClient, apiserver.Namespace, s.Request.PolicyName)
	if err != nil {
		s.Response.Status.Code = msgs.Error
		s.Response.Status.Msg = fmt.Sprintf("policy %s not found", s.Request.PolicyName)
		return &PgScheduleSpec{}
	}

	if s.Request.Secret == "" {
		s.Request.Secret = cluster.Spec.PrimarySecretName
	}
	schedule := &PgScheduleSpec{
		Name:      name,
		Cluster:   cluster.Name,
		Version:   "v1",
		Created:   time.Now().Format(time.RFC3339),
		Schedule:  s.Request.Schedule,
		Type:      s.Request.ScheduleType,
		Namespace: apiserver.Namespace,
		Policy: Policy{
			Name:        s.Request.PolicyName,
			Database:    s.Request.Database,
			Secret:      s.Request.Secret,
			ImagePrefix: apiserver.Pgo.Pgo.COImagePrefix,
			ImageTag:    apiserver.Pgo.Pgo.COImageTag,
		},
	}
	return schedule
}

//  CreateSchedule
func CreateSchedule(request *msgs.CreateScheduleRequest) msgs.CreateScheduleResponse {
	log.Debugf("Create schedule called: %s", request.ClusterName)
	sr := &scheduleRequest{
		Request: request,
		Response: &msgs.CreateScheduleResponse{
			Status: msgs.Status{
				Code: msgs.Ok,
				Msg:  "",
			},
			Results: make([]string, 0),
		},
	}

	log.Debug("Getting cluster")
	var selector string
	if sr.Request.ClusterName != "" {
		selector = fmt.Sprintf("%s=%s", util.LABEL_PG_CLUSTER, sr.Request.ClusterName)
	} else if sr.Request.Selector != "" {
		selector = sr.Request.Selector
	}

	clusterList := crv1.PgclusterList{}
	err := kubeapi.GetpgclustersBySelector(apiserver.RESTClient, &clusterList, selector, apiserver.Namespace)
	if err != nil {
		sr.Response.Status.Code = msgs.Error
		sr.Response.Status.Msg = fmt.Sprintf("Could not get cluster via selector: %s", err)
		return *sr.Response
	}

	log.Debug("Making schedules")
	var schedules []*PgScheduleSpec
	for _, cluster := range clusterList.Items {
		switch sr.Request.ScheduleType {
		case "pgbackrest":
			schedule := sr.createBackRestSchedule(&cluster)
			schedules = append(schedules, schedule)
		case "pgbasebackup":
			schedule := sr.createBaseBackupSchedule(&cluster)
			schedules = append(schedules, schedule)
		case "policy":
			schedule := sr.createPolicySchedule(&cluster)
			schedules = append(schedules, schedule)
		default:
			sr.Response.Status.Code = msgs.Error
			sr.Response.Status.Msg = fmt.Sprintf("Schedule type unknown: %s", sr.Request.ScheduleType)
			return *sr.Response
		}

		if sr.Response.Status.Code == msgs.Error {
			return *sr.Response
		}
	}

	log.Debug("Marshalling schedules")
	for _, schedule := range schedules {
		log.Debug(schedule.Name, schedule.Cluster)
		blob, err := json.Marshal(schedule)
		if err != nil {
			sr.Response.Status.Code = msgs.Error
			sr.Response.Status.Msg = err.Error()
		}

		log.Debug("Getting configmap..")
		_, exists := kubeapi.GetConfigMap(apiserver.Clientset, schedule.Name, schedule.Namespace)
		if exists {
			sr.Response.Status.Code = msgs.Error
			sr.Response.Status.Msg = fmt.Sprintf("Schedule %s already exists", schedule.Name)
			return *sr.Response
		}

		labels := make(map[string]string)
		labels["pg-cluster"] = schedule.Cluster
		labels["crunchy-scheduler"] = "true"

		data := make(map[string]string)
		data[schedule.Name] = string(blob)

		configmap := &v1.ConfigMap{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:   schedule.Name,
				Labels: labels,
			},
			Data: data,
		}

		log.Debug("Creating configmap..")
		err = kubeapi.CreateConfigMap(apiserver.Clientset, configmap, schedule.Namespace)
		if err != nil {
			sr.Response.Status.Code = msgs.Error
			sr.Response.Status.Msg = err.Error()
			return *sr.Response
		}

		msg := fmt.Sprintf("created schedule %s for cluster %s", configmap.ObjectMeta.Name, schedule.Cluster)
		sr.Response.Results = append(sr.Response.Results, msg)
	}
	return *sr.Response
}

//  DeleteSchedule ...
func DeleteSchedule(request *msgs.DeleteScheduleRequest) msgs.DeleteScheduleResponse {
	log.Debug("Deleted schedule called")

	sr := &msgs.DeleteScheduleResponse{
		Status: msgs.Status{
			Code: msgs.Ok,
			Msg:  "",
		},
		Results: make([]string, 0),
	}

	if request.ScheduleName == "" && request.ClusterName == "" && request.Selector == "" {
		sr.Status.Code = msgs.Error
		sr.Status.Msg = fmt.Sprintf("Cluster name, schedule name or selector must be provided")
		return *sr
	}

	schedules := []string{}
	var err error
	if request.ScheduleName != "" {
		schedules = append(schedules, request.ScheduleName)
	} else {
		schedules, err = getSchedules(request.ClusterName, request.Selector)
		if err != nil {
			sr.Status.Code = msgs.Error
			sr.Status.Msg = err.Error()
			return *sr
		}
	}

	log.Debug("Deleting configMaps")
	for _, schedule := range schedules {
		err := kubeapi.DeleteConfigMap(apiserver.Clientset, schedule, apiserver.Namespace)
		if err != nil {
			sr.Status.Code = msgs.Error
			sr.Status.Msg = fmt.Sprintf("Could not delete ConfigMap %s: %s", schedule, err)
			return *sr
		}
		msg := fmt.Sprintf("deleted schedule %s", schedule)
		sr.Results = append(sr.Results, msg)
	}

	return *sr
}

//  ShowSchedule ...
func ShowSchedule(request *msgs.ShowScheduleRequest) msgs.ShowScheduleResponse {
	log.Debug("Show schedule called")

	sr := &msgs.ShowScheduleResponse{
		Status: msgs.Status{
			Code: msgs.Ok,
			Msg:  "",
		},
		Results: make([]string, 0),
	}

	if request.ScheduleName == "" && request.ClusterName == "" && request.Selector == "" {
		sr.Status.Code = msgs.Error
		sr.Status.Msg = fmt.Sprintf("Cluster name, schedule name or selector must be provided")
		return *sr
	}

	schedules := []string{}
	var err error
	if request.ScheduleName != "" {
		schedules = append(schedules, request.ScheduleName)
	} else {
		schedules, err = getSchedules(request.ClusterName, request.Selector)
		if err != nil {
			sr.Status.Code = msgs.Error
			sr.Status.Msg = err.Error()
			return *sr
		}
	}

	log.Debug("Parsing configMaps")
	for _, schedule := range schedules {
		cm, exists := kubeapi.GetConfigMap(apiserver.Clientset, schedule, apiserver.Namespace)
		if !exists {
			sr.Status.Code = msgs.Error
			sr.Status.Msg = fmt.Sprintf("Could not delete ConfigMap %s: %s", schedule, err)
			return *sr
		}

		var blob PgScheduleSpec
		log.Debug(cm.Data[schedule])
		if err := json.Unmarshal([]byte(cm.Data[schedule]), &blob); err != nil {
			sr.Status.Code = msgs.Error
			sr.Status.Msg = fmt.Sprintf("Could not parse schedule json %s: %s", schedule, err)
			return *sr
		}

		results := fmt.Sprintf("%s:\n\tschedule: %s\n\tschedule-type: %s", blob.Name, blob.Schedule, blob.Type)
		if blob.Type == "pgbackrest" {
			results += fmt.Sprintf("\n\tbackup-type: %s", blob.PGBackRest.Type)
		} else if blob.Type == "pgbasebackup" {
			results += fmt.Sprintf("\n\tbackup-volume: %s", blob.PGBaseBackup.BackupVolume)
		}
		sr.Results = append(sr.Results, results)
	}
	return *sr
}

func getSchedules(clusterName, selector string) ([]string, error) {
	schedules := []string{}
	label := "crunchy-scheduler=true"
	if clusterName == "all" {
	} else if clusterName != "" {
		label += fmt.Sprintf(",pg-cluster=%s", clusterName)
	}

	if selector != "" {
		label += fmt.Sprintf(",%s", selector)
	}

	log.Debugf("Finding configMaps with selector: %s", label)
	list, ok := kubeapi.ListConfigMap(apiserver.Clientset, label, apiserver.Namespace)
	if !ok {
		return nil, fmt.Errorf("No schedules found for selector: %s", label)
	}

	for _, cm := range list.Items {
		schedules = append(schedules, cm.Name)
	}

	return schedules, nil
}
