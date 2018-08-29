package api

/*
 Copyright 2017-2018 Crunchy Data Solutions, Inc.
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
	log "github.com/Sirupsen/logrus"
	msgs "github.com/crunchydata/postgres-operator/apiservermsgs"
	"github.com/crunchydata/postgres-operator/util"
	"net/http"
	"strconv"
)

func ScaleDownCluster(httpclient *http.Client, APIServerURL, clusterName, ScaleDownTarget string, DeleteData bool, BasicAuthUsername, BasicAuthPassword string) (msgs.ScaleDownResponse, error) {

	var response msgs.ScaleDownResponse
	url := APIServerURL + "/scaledown/" + clusterName + "?version=" + msgs.PGO_VERSION + "&" + util.LABEL_REPLICA_NAME + "=" + ScaleDownTarget + "&" + util.LABEL_DELETE_DATA + "=" + strconv.FormatBool(DeleteData)
	log.Debug(url)

	action := "GET"
	req, err := http.NewRequest(action, url, nil)
	if err != nil {
		return response, err
	}

	req.SetBasicAuth(BasicAuthUsername, BasicAuthPassword)
	resp, err := httpclient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		fmt.Println("Error: Do: ", err)
		return response, err
	}
	log.Debugf("%v\n", resp)
	err = StatusCheck(resp)
	if err != nil {
		return response, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("%v\n", resp.Body)
		log.Println(err)
		return response, err
	}

	return response, err

}

func ScaleQuery(httpclient *http.Client, APIServerURL, arg, BasicAuthUsername, BasicAuthPassword string) (msgs.ScaleQueryResponse, error) {

	var response msgs.ScaleQueryResponse

	url := APIServerURL + "/scale/" + arg + "?version=" + msgs.PGO_VERSION
	log.Debug(url)

	action := "GET"

	req, err := http.NewRequest(action, url, nil)
	if err != nil {
		return response, err
	}

	req.SetBasicAuth(BasicAuthUsername, BasicAuthPassword)

	resp, err := httpclient.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()
	log.Debugf("%v\n", resp)
	err = StatusCheck(resp)
	if err != nil {
		return response, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("%v\n", resp.Body)
		fmt.Println("Error: ", err)
		log.Println(err)
		return response, err
	}

	return response, err

}

func ScaleCluster(httpclient *http.Client, APIServerURL, arg string, ReplicaCount int, ContainerResources, StorageConfig, NodeLabel, CCPImageTag, ServiceType, BasicAuthUsername, BasicAuthPassword string) (msgs.ClusterScaleResponse, error) {

	var response msgs.ClusterScaleResponse

	url := APIServerURL + "/clusters/scale/" + arg + "?replica-count=" + strconv.Itoa(ReplicaCount) + "&resources-config=" + ContainerResources + "&storage-config=" + StorageConfig + "&node-label=" + NodeLabel + "&version=" + msgs.PGO_VERSION + "&ccp-image-tag=" + CCPImageTag + "&service-type=" + ServiceType
	log.Debug(url)

	action := "GET"

	req, err := http.NewRequest(action, url, nil)
	if err != nil {
		return response, err
	}

	req.SetBasicAuth(BasicAuthUsername, BasicAuthPassword)

	resp, err := httpclient.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()
	log.Debugf("%v\n", resp)
	err = StatusCheck(resp)
	if err != nil {
		return response, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("%v\n", resp.Body)
		fmt.Println("Error: ", err)
		log.Println(err)
		return response, err
	}

	return response, err

}