package server

import (
	"encoding/json"
	"github.com/c12s/lunar-gateway/model"
	bPb "github.com/c12s/scheme/blackhole"
	cPb "github.com/c12s/scheme/celestial"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	BLACKHOLE = "blackhole"
	CELESTIAL = "celestial"

	all = "all"
	any = "any"

	file   = "file"
	env    = "env"
	action = "action"

	at_once        = "AtOnce"
	rolling_update = "RollingUpdate"

	compare = "compare"
	labels  = "labels"
	sep     = ":"
	kind    = "kind"

	top  = "top"
	from = "from"
	to   = "to"

	user = "user"
)

func merge(m1, m2 map[string]string) {
	for k, v := range m2 {
		m1[k] = v
	}
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to encode a JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(body)
	if err != nil {
		log.Printf("Failed to write the response body: %v", err)
		return
	}
}

func sendErrorMessage(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(status)
	io.WriteString(w, msg)
}

func cKind(kind string) bPb.CompareKind {
	switch kind {
	case all:
		return bPb.CompareKind_ALL
	case any:
		return bPb.CompareKind_ANY
	default:
		return -1
	}
}

func pKind(kind string) bPb.PayloadKind {
	switch kind {
	case file:
		return bPb.PayloadKind_FILE
	case env:
		return bPb.PayloadKind_ENV
	case action:
		return bPb.PayloadKind_ACTION
	default:
		return -1
	}
}

func sKind(kind string) bPb.StrategyKind {
	switch kind {
	case at_once:
		return bPb.StrategyKind_AT_ONCE
	case rolling_update:
		return bPb.StrategyKind_ROLLING_UPDATE
	default:
		return -1
	}
}

func tKind(kind string) bPb.TaskKind {
	switch kind {
	case "Configs":
		return bPb.TaskKind_CONFIGS
	case "Secrets":
		return bPb.TaskKind_SECRETS
	case "Actions":
		return bPb.TaskKind_ACTIONS
	case "Namespaces":
		return bPb.TaskKind_NAMESPACES
	default:
		return -1
	}
}

func mutateToProto(data *model.MutateRequest) *bPb.PutReq {
	tasks := []*bPb.PutTask{}
	for _, region := range data.Regions {
		for _, cluster := range region.Clusters {
			labels := map[string]string{}
			for k, v := range cluster.Selector.Labels {
				labels[k] = v
			}

			payload := []*bPb.Payload{}
			for _, entry := range cluster.Payload {
				values := map[string]string{}
				for k, v := range entry.Content {
					values[k] = v
				}
				pld := &bPb.Payload{
					Kind:  pKind(entry.Kind),
					Value: values,
				}
				payload = append(payload, pld)
			}

			task := &bPb.PutTask{
				RegionId:  region.ID,
				ClusterId: cluster.ID,
				Strategy: &bPb.Strategy{
					Type: sKind(cluster.Strategy.Type),
					Kind: cluster.Strategy.Kind,
				},
				Selector: &bPb.Selector{
					Kind:   cKind(cluster.Selector.Compare[kind]),
					Labels: labels,
				},
				Payload: payload,
			}
			tasks = append(tasks, task)
		}
	}

	return &bPb.PutReq{
		Version: data.Version,
		UserId:  data.Request,
		Kind:    tKind(data.Kind),
		Mtdata: &bPb.Metadata{
			TaskName:            data.MTData.TaskName,
			Timestamp:           data.MTData.Timestamp,
			Namespace:           data.MTData.Namespace,
			ForceNamespaceQueue: data.MTData.ForceNSQueue,
			Queue:               data.MTData.Queue,
		},
		Tasks: tasks,
	}
}

func mutateNSToProto(data *model.NMutateRequest) *bPb.PutReq {
	extras := map[string]string{}
	for k, v := range data.Labels {
		extras[k] = v
	}

	// Add namespace name to the extras
	nkey := strings.Join([]string{data.MTData.Namespace, "name"}, "__")
	extras[nkey] = data.Name

	return &bPb.PutReq{
		Version: data.Version,
		UserId:  data.Request,
		Kind:    tKind(data.Kind),
		Mtdata: &bPb.Metadata{
			TaskName:            data.MTData.TaskName,
			Timestamp:           data.MTData.Timestamp,
			Namespace:           data.MTData.Namespace,
			ForceNamespaceQueue: data.MTData.ForceNSQueue,
			Queue:               data.MTData.Queue,
		},
		Extras: labels,
	}
}

func listToProto(data map[string][]string) *cPb.ListReq {
	extras := map[string]string{}
	for k, v := range data {
		if k == labels {
			value := strings.Join(v, ",")
			extras[labels] = value
		} else {
			extras[compare] = v[0]
		}
	}
	return &cPb.ListReq{
		Extras: extras,
	}
}

func RequestToProto(req interface{}, data interface{}) {
	switch castReq := req.(type) {
	case model.MutateRequest:
		data = mutateToProto(&castReq)
	case model.NMutateRequest:
		data = mutateNSToProto(&castReq)
	case map[string][]string:
		data = listToProto(castReq)
	default:
		data = nil
	}
}
