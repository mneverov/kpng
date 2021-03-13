/*
Copyright 2021 The Kubernetes Authors.

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

package endpoints

import (
	"flag"

	v1 "k8s.io/api/core/v1"

	"m.cluseau.fr/kpng/pkg/api/localnetv1"
	"m.cluseau.fr/kpng/pkg/proxy"
	"m.cluseau.fr/kpng/pkg/proxystore"
)

type nodeEventHandler struct{ eventHandler }

var (
	myNodeName = flag.String("node-name", "", "Node name override")

	nodeLabelGlobs      = flag.String("with-node-labels", "", "node labels to include")
	nodeAnnotationGlobs = flag.String("with-node-annotations", "", "node annotations to include")
)

func (h nodeEventHandler) OnAdd(obj interface{}) {
	node := obj.(*v1.Node)

	// keep only what we want
	n := &localnetv1.Node{
		Name:        node.Name,
		Labels:      globsFilter(node.Labels, *nodeLabelGlobs),
		Annotations: globsFilter(node.Annotations, *nodeAnnotationGlobs),
	}

	h.s.Update(func(tx *proxystore.Tx) {
		tx.SetNode(n)

		if !proxy.ManageEndpointSlices {
			// endpoints => need to update all matching topologies
			toSet := make([]*localnetv1.EndpointInfo, 0)
			tx.Each(proxystore.Endpoints, func(kv *proxystore.KV) bool {
				if kv.Endpoint.NodeName == n.Name {
					kv.Endpoint.Topology = n.Labels
					toSet = append(toSet, kv.Endpoint)
				}
				return true
			})

			for _, ei := range toSet {
				tx.SetEndpoint(ei)
			}
		}

		h.updateSync(proxystore.Nodes, tx)
	})
}

func (h nodeEventHandler) OnUpdate(oldObj, newObj interface{}) {
	// same as adding
	h.OnAdd(newObj)
}

func (h nodeEventHandler) OnDelete(oldObj interface{}) {
	node := oldObj.(*v1.Node)

	h.s.Update(func(tx *proxystore.Tx) {
		tx.DelNode(node.Name)
		h.updateSync(proxystore.Nodes, tx)
	})
}
