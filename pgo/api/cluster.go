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
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	msgs "github.com/crunchydata/postgres-operator/apiservermsgs"
	"net/http"
	"strconv"
)

func ShowCluster(httpclient *http.Client, arg, selector, ccpimagetag string, SessionCredentials *msgs.BasicAuthCredentials) (msgs.ShowClusterResponse, error) {

	var response msgs.ShowClusterResponse

	url := SessionCredentials.APIServerURL + "/clusters/" + arg + "?selector=" + selector + "&version=" + msgs.PGO_VERSION + "&ccpimagetag=" + ccpimagetag

	log.Debug("show cluster called [" + url + "]")

	action := "GET"
	req, err := http.NewRequest(action, url, nil)

	if err != nil {
		return response, err
	}

	req.SetBasicAuth(SessionCredentials.Username, SessionCredentials.Password)

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

func DeleteCluster(httpclient *http.Client, arg, selector string, SessionCredentials *msgs.BasicAuthCredentials, deleteData, deleteBackups bool) (msgs.DeleteClusterResponse, error) {

	var response msgs.DeleteClusterResponse

	log.Debug("deleting cluster " + arg + " with delete-data " + strconv.FormatBool(deleteData))

	url := SessionCredentials.APIServerURL + "/clustersdelete/" + arg + "?selector=" + selector + "&delete-data=" + strconv.FormatBool(deleteData) + "&delete-backups=" + strconv.FormatBool(deleteBackups) + "&version=" + msgs.PGO_VERSION

	log.Debug("delete cluster called [" + url + "]")

	action := "GET"
	req, err := http.NewRequest(action, url, nil)
	if err != nil {
		return response, err
	}

	req.SetBasicAuth(SessionCredentials.Username, SessionCredentials.Password)

	resp, err := httpclient.Do(req)
	if err != nil {
		fmt.Println("Error: Do: ", err)
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

func CreateCluster(httpclient *http.Client, SessionCredentials *msgs.BasicAuthCredentials, request *msgs.CreateClusterRequest) (msgs.CreateClusterResponse, error) {

	var response msgs.CreateClusterResponse

	jsonValue, _ := json.Marshal(request)
	url := SessionCredentials.APIServerURL + "/clusters"
	log.Debug("createCluster called...[" + url + "]")

	action := "POST"
	req, err := http.NewRequest(action, url, bytes.NewBuffer(jsonValue))
	if err != nil {
		return response, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(SessionCredentials.Username, SessionCredentials.Password)

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
		log.Println(err)
		return response, err
	}

	return response, err
}
