package helm

import (
	"reflect"
	"testing"

	"github.com/keel-hq/keel/internal/policy"
	"github.com/keel-hq/keel/types"
	"github.com/keel-hq/keel/util/image"
	"k8s.io/helm/pkg/chartutil"
)

var chartValuesA = `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: all  
  trigger: poll  
  images:
    - repository: image.repository
      tag: image.tag

`

func mustParse(name string) *image.Reference {
	img, err := image.Parse(name)
	if err != nil {
		panic(err)
	}
	return img
}

func Test_getImages(t *testing.T) {
	vals, _ := chartutil.ReadValues([]byte(chartValuesA))
	img, _ := image.Parse("gcr.io/v2-namespace/hello-world:1.1.0")

	promVals, _ := chartutil.ReadValues([]byte(promChartValues))

	type args struct {
		vals chartutil.Values
	}
	tests := []struct {
		name    string
		args    args
		want    []*types.TrackedImage
		wantErr bool
	}{
		{
			name: "hello-world image",
			args: args{
				vals: vals,
			},
			want: []*types.TrackedImage{
				&types.TrackedImage{
					Image:   img,
					Trigger: types.TriggerTypePoll,
					Policy:  policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
				},
			},
			wantErr: false,
		},
		{
			name: "prom config from https://raw.githubusercontent.com/helm/charts/master/stable/prometheus-operator/values.yaml",
			args: args{
				vals: promVals,
			},
			want: []*types.TrackedImage{
				&types.TrackedImage{
					Image:   mustParse("quay.io/prometheus/alertmanager:v0.16.2"),
					Trigger: types.TriggerTypePoll,
					Policy:  policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
				},
				&types.TrackedImage{
					Image:   mustParse("quay.io/coreos/prometheus-operator:v0.29.0"),
					Trigger: types.TriggerTypePoll,
					Policy:  policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
				},
				&types.TrackedImage{
					Image:   mustParse("quay.io/prometheus/prometheus:v2.7.2"),
					Trigger: types.TriggerTypePoll,
					Policy:  policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getImages(tt.args.vals)
			if (err != nil) != tt.wantErr {
				t.Errorf("getImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getImages() = %v, want %v", got, tt.want)
			}
		})
	}
}

var promChartValues = `# Default values for prometheus-operator.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

# keel config
keel:
  policy: all
  trigger: poll
  images:
  - repository: alertmanager.alertmanagerSpec.image.repository
    tag: alertmanager.alertmanagerSpec.image.tag
  - repository: prometheusOperator.image.repository
    tag: prometheusOperator.image.tag
  - repository: prometheus.prometheusSpec.image.repository
    tag: prometheus.prometheusSpec.image.tag

## Provide a name in place of prometheus-operator for labels
##
nameOverride: ""

## Provide a name to substitute for the full names of resources
##
fullnameOverride: ""

## Labels to apply to all resources
##
commonLabels: {}
# scmhash: abc123
# myLabel: aakkmd

## Create default rules for monitoring the cluster
##
defaultRules:
  create: true
  rules:
    alertmanager: true
    etcd: true
    general: true
    k8s: true
    kubeApiserver: true
    kubePrometheusNodeAlerting: true
    kubePrometheusNodeRecording: true
    kubeScheduler: true
    kubernetesAbsent: true
    kubernetesApps: true
    kubernetesResources: true
    kubernetesStorage: true
    kubernetesSystem: true
    node: true
    prometheusOperator: true
    prometheus: true
  ## Labels for default rules
  labels: {}
  ## Annotations for default rules
  annotations: {}

## Provide custom recording or alerting rules to be deployed into the cluster.
##
additionalPrometheusRules: []
#  - name: my-rule-file
#    groups:
#      - name: my_group
#        rules:
#        - record: my_record
#          expr: 100 * my_record

##
global:
  rbac:
    create: true
    pspEnabled: true

  ## Reference to one or more secrets to be used when pulling images
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
  ##
  imagePullSecrets: []
  # - name: "image-pull-secret"

## Configuration for alertmanager
## ref: https://prometheus.io/docs/alerting/alertmanager/
##
alertmanager:

  ## Deploy alertmanager
  ##
  enabled: true

  ## Service account for Alertmanager to use.
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
  ##
  serviceAccount:
    create: true
    name: ""

  ## Configure pod disruption budgets for Alertmanager
  ## ref: https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget
  ## This configuration is immutable once created and will require the PDB to be deleted to be changed
  ## https://github.com/kubernetes/kubernetes/issues/45398
  ##
  podDisruptionBudget:
    enabled: false
    minAvailable: 1
    maxUnavailable: ""

  ## Alertmanager configuration directives
  ## ref: https://prometheus.io/docs/alerting/configuration/#configuration-file
  ##      https://prometheus.io/webtools/alerting/routing-tree-editor/
  ##
  config:
    global:
      resolve_timeout: 5m
    route:
      group_by: ['job']
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 12h
      receiver: 'null'
      routes:
      - match:
          alertname: Watchdog
        receiver: 'null'
    receivers:
    - name: 'null'

  ## Alertmanager template files to format alerts
  ## ref: https://prometheus.io/docs/alerting/notifications/
  ##      https://prometheus.io/docs/alerting/notification_examples/
  ##
  templateFiles: {}
  #
  # An example template:
  #   template_1.tmpl: |-
  #       {{ define "cluster" }}{{ .ExternalURL | reReplaceAll ".*alertmanager\\.(.*)" "$1" }}{{ end }}
  #
  #       {{ define "slack.myorg.text" }}
  #       {{- $root := . -}}
  #       {{ range .Alerts }}
  #         *Alert:* {{ .Annotations.summary }} -
  #         *Cluster:*  {{ template "cluster" $root }}
  #         *Description:* {{ .Annotations.description }}
  #         *Graph:* <{{ .GeneratorURL }}|:chart_with_upwards_trend:>
  #         *Runbook:* <{{ .Annotations.runbook }}|:spiral_note_pad:>
  #         *Details:*
  #           {{ range .Labels.SortedPairs }} • *{{ .Name }}:* 
  #           {{ end }}

  ingress:
    enabled: false

    annotations: {}

    labels: {}

    ## Hosts must be provided if Ingress is enabled.
    ##
    hosts: []
      # - alertmanager.domain.com

    ## Paths to use for ingress rules - one path should match the alertmanagerSpec.routePrefix
    ##
    paths: []
    # - /

    ## TLS configuration for Alertmanager Ingress
    ## Secret must be manually created in the namespace
    ##
    tls: []
    # - secretName: alertmanager-general-tls
    #   hosts:
    #   - alertmanager.example.com

  ## Configuration for Alertmanager service
  ##
  service:
    annotations: {}
    labels: {}
    clusterIP: ""

  ## Port to expose on each node
  ## Only used if service.type is 'NodePort'
  ##
    nodePort: 30903
  ## List of IP addresses at which the Prometheus server service is available
  ## Ref: https://kubernetes.io/docs/user-guide/services/#external-ips
  ##
    externalIPs: []
    loadBalancerIP: ""
    loadBalancerSourceRanges: []
    ## Service type
    ##
    type: ClusterIP

  ## If true, create a serviceMonitor for alertmanager
  ##
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    selfMonitor: true

  ## Settings affecting alertmanagerSpec
  ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#alertmanagerspec
  ##
  alertmanagerSpec:
    ## Standard object’s metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
    ## Metadata Labels and Annotations gets propagated to the Alertmanager pods.
    ##
    podMetadata: {}

    ## Image of Alertmanager
    ##
    image:
      repository: quay.io/prometheus/alertmanager
      tag: v0.16.2

    ## Secrets is a list of Secrets in the same namespace as the Alertmanager object, which shall be mounted into the
    ## Alertmanager Pods. The Secrets are mounted into /etc/alertmanager/secrets/.
    ##
    secrets: []

    ## ConfigMaps is a list of ConfigMaps in the same namespace as the Alertmanager object, which shall be mounted into the Alertmanager Pods.
    ## The ConfigMaps are mounted into /etc/alertmanager/configmaps/.
    ##
    configMaps: []

    ## Log level for Alertmanager to be configured with.
    ##
    logLevel: info

    ## Size is the expected size of the alertmanager cluster. The controller will eventually make the size of the
    ## running cluster equal to the expected size.
    replicas: 1

    ## Time duration Alertmanager shall retain data for. Default is '120h', and must match the regular expression
    ## [0-9]+(ms|s|m|h) (milliseconds seconds minutes hours).
    ##
    retention: 120h

    ## Storage is the definition of how storage will be used by the Alertmanager instances.
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/user-guides/storage.md
    ##
    storage: {}
    # volumeClaimTemplate:
    #   spec:
    #     storageClassName: gluster
    #     accessModes: ["ReadWriteOnce"]
    #     resources:
    #       requests:
    #         storage: 50Gi
    #   selector: {}


    ## 	The external URL the Alertmanager instances will be available under. This is necessary to generate correct URLs. This is necessary if Alertmanager is not served from root of a DNS name.	string	false
    ##
    externalUrl:

    ## 	The route prefix Alertmanager registers HTTP handlers for. This is useful, if using ExternalURL and a proxy is rewriting HTTP routes of a request, and the actual ExternalURL is still true,
    ## but the server serves requests under a different route prefix. For example for use with kubectl proxy.
    ##
    routePrefix: /

    ## If set to true all actions on the underlying managed objects are not going to be performed, except for delete actions.
    ##
    paused: false

    ## Define which Nodes the Pods are scheduled on.
    ## ref: https://kubernetes.io/docs/user-guide/node-selection/
    ##
    nodeSelector: {}

    ## Define resources requests and limits for single Pods.
    ## ref: https://kubernetes.io/docs/user-guide/compute-resources/
    ##
    resources: {}
    # requests:
    #   memory: 400Mi

    ## Pod anti-affinity can prevent the scheduler from placing Prometheus replicas on the same node.
    ## The default value "soft" means that the scheduler should *prefer* to not schedule two replica pods onto the same node but no guarantee is provided.
    ## The value "hard" means that the scheduler is *required* to not schedule two replica pods onto the same node.
    ## The value "" will disable pod anti-affinity so that no anti-affinity rules will be configured.
    ##
    podAntiAffinity: ""

    ## If anti-affinity is enabled sets the topologyKey to use for anti-affinity.
    ## This can be changed to, for example, failure-domain.beta.kubernetes.io/zone
    ##
    podAntiAffinityTopologyKey: kubernetes.io/hostname

    ## If specified, the pod's tolerations.
    ## ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
    ##
    tolerations: []
    # - key: "key"
    #   operator: "Equal"
    #   value: "value"
    #   effect: "NoSchedule"

    ## SecurityContext holds pod-level security attributes and common container settings.
    ## This defaults to non root user with uid 1000 and gid 2000.	*v1.PodSecurityContext	false
    ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
    ##
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 2000

    ## ListenLocal makes the Alertmanager server listen on loopback, so that it does not bind against the Pod IP.
    ## Note this is only for the Alertmanager UI, not the gossip communication.
    ##
    listenLocal: false

    ## Containers allows injecting additional containers. This is meant to allow adding an authentication proxy to an Alertmanager pod.
    ##
    containers: []

    ## Priority class assigned to the Pods
    ##
    priorityClassName: ""

    ## AdditionalPeers allows injecting a set of additional Alertmanagers to peer with to form a highly available cluster.
    ##
    additionalPeers: []

## Using default values from https://github.com/helm/charts/blob/master/stable/grafana/values.yaml
##
grafana:
  enabled: true

  ## Deploy default dashboards.
  ##
  defaultDashboardsEnabled: true

  adminPassword: prom-operator

  ingress:
    ## If true, Grafana Ingress will be created
    ##
    enabled: false

    ## Annotations for Grafana Ingress
    ##
    annotations: {}
      # kubernetes.io/ingress.class: nginx
      # kubernetes.io/tls-acme: "true"

    ## Labels to be added to the Ingress
    ##
    labels: {}

    ## Hostnames.
    ## Must be provided if Ingress is enable.
    ##
    # hosts:
    #   - grafana.domain.com
    hosts: []

    ## Path for grafana ingress
    path: /

    ## TLS configuration for grafana Ingress
    ## Secret must be manually created in the namespace
    ##
    tls: []
    # - secretName: grafana-general-tls
    #   hosts:
    #   - grafana.example.com

  sidecar:
    dashboards:
      enabled: true
      label: grafana_dashboard
    datasources:
      enabled: true
      defaultDatasourceEnabled: true
      label: grafana_datasource

  extraConfigmapMounts: []
  # - name: certs-configmap
  #   mountPath: /etc/grafana/ssl/
  #   configMap: certs-configmap
  #   readOnly: true

  ## Configure additional grafana datasources
  ## ref: http://docs.grafana.org/administration/provisioning/#datasources
  additionalDataSources: []
  # - name: prometheus-sample
  #   access: proxy
  #   basicAuth: true
  #   basicAuthPassword: pass
  #   basicAuthUser: daco
  #   editable: false
  #   jsonData:
  #       tlsSkipVerify: true
  #   orgId: 1
  #   type: prometheus
  #   url: https://prometheus.svc:9090
  #   version: 1

  ## If true, create a serviceMonitor for grafana
  ##
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    selfMonitor: true

## Component scraping the kube api server
##
kubeApiServer:
  enabled: true
  tlsConfig:
    serverName: kubernetes
    insecureSkipVerify: false

  ## If your API endpoint address is not reachable (as in AKS) you can replace it with the kubernetes service
  ##
  relabelings: []
  # - sourceLabels:
  #     - __meta_kubernetes_namespace
  #     - __meta_kubernetes_service_name
  #     - __meta_kubernetes_endpoint_port_name
  #   action: keep
  #   regex: default;kubernetes;https
  # - targetLabel: __address__
  #   replacement: kubernetes.default.svc:443

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    jobLabel: component
    selector:
      matchLabels:
        component: apiserver
        provider: kubernetes

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

## Component scraping the kubelet and kubelet-hosted cAdvisor
##
kubelet:
  enabled: true
  namespace: kube-system

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## Enable scraping the kubelet over https. For requirements to enable this see
    ## https://github.com/coreos/prometheus-operator/issues/926
    ##
    https: true

    ## Metric relabellings to apply to samples before ingestion
    ##
    cAdvisorMetricRelabelings: []
    # - sourceLabels: [__name__, image]
    #   separator: ;
    #   regex: container_([a-z_]+);
    #   replacement: $1
    #   action: drop
    # - sourceLabels: [__name__]
    #   separator: ;
    #   regex: container_(network_tcp_usage_total|network_udp_usage_total|tasks_state|cpu_load_average_10s)
    #   replacement: $1
    #   action: drop

    # 	relabel configs to apply to samples before ingestion.
    ##
    cAdvisorRelabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Component scraping the kube controller manager
##
kubeControllerManager:
  enabled: true

  ## If your kube controller manager is not deployed as a pod, specify IPs it can be found on
  ##
  endpoints: []
  # - 10.141.4.22
  # - 10.141.4.23
  # - 10.141.4.24

  ## If using kubeControllerManager.endpoints only the port and targetPort are used
  ##
  service:
    port: 10252
    targetPort: 10252
    selector:
      component: kube-controller-manager

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## Enable scraping kube-controller-manager over https.
    ## Requires proper certs (not self-signed) and delegated authentication/authorization checks
    ##
    https: false

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Component scraping coreDns. Use either this or kubeDns
##
coreDns:
  enabled: true
  service:
    port: 9153
    targetPort: 9153
    selector:
      k8s-app: kube-dns
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Component scraping kubeDns. Use either this or coreDns
##
kubeDns:
  enabled: false
  service:
    selector:
      k8s-app: kube-dns
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Component scraping etcd
##
kubeEtcd:
  enabled: true

  ## If your etcd is not deployed as a pod, specify IPs it can be found on
  ##
  endpoints: []
  # - 10.141.4.22
  # - 10.141.4.23
  # - 10.141.4.24

  ## Etcd service. If using kubeEtcd.endpoints only the port and targetPort are used
  ##
  service:
    port: 2379
    targetPort: 2379
    selector:
      component: etcd

  ## Configure secure access to the etcd cluster by loading a secret into prometheus and
  ## specifying security configuration below. For example, with a secret named etcd-client-cert
  ##
  ## serviceMonitor:
  ##   scheme: https
  ##   insecureSkipVerify: false
  ##   serverName: localhost
  ##   caFile: /etc/prometheus/secrets/etcd-client-cert/etcd-ca
  ##   certFile: /etc/prometheus/secrets/etcd-client-cert/etcd-client
  ##   keyFile: /etc/prometheus/secrets/etcd-client-cert/etcd-client-key
  ##
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    scheme: http
    insecureSkipVerify: false
    serverName: ""
    caFile: ""
    certFile: ""
    keyFile: ""

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace


## Component scraping kube scheduler
##
kubeScheduler:
  enabled: true

  ## If your kube scheduler is not deployed as a pod, specify IPs it can be found on
  ##
  endpoints: []
  # - 10.141.4.22
  # - 10.141.4.23
  # - 10.141.4.24

  ## If using kubeScheduler.endpoints only the port and targetPort are used
  ##
  service:
    port: 10251
    targetPort: 10251
    selector:
      component: kube-scheduler

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    ## Enable scraping kube-controller-manager over https.
    ## Requires proper certs (not self-signed) and delegated authentication/authorization checks
    ##
    https: false

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Component scraping kube state metrics
##
kubeStateMetrics:
  enabled: true
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - action: keep
    #   regex: 'kube_(daemonset|deployment|pod|namespace|node|statefulset).+'
    #   sourceLabels: [__name__]

    # 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Configuration for kube-state-metrics subchart
##
kube-state-metrics:
  rbac:
    create: true
  podSecurityPolicy:
    enabled: true

## Deploy node exporter as a daemonset to all nodes
##
nodeExporter:
  enabled: true

  ## Use the value configured in prometheus-node-exporter.podLabels
  ##
  jobLabel: jobLabel

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""

    ## 	metric relabel configs to apply to samples before ingestion.
    ##
    metricRelabelings: []
    # - sourceLabels: [__name__]
    #   separator: ;
    #   regex: ^node_mountstats_nfs_(event|operations|transport)_.+
    #   replacement: $1
    #   action: drop

    ## 	relabel configs to apply to samples before ingestion.
    ##
    relabelings: []
    # - sourceLabels: [__meta_kubernetes_pod_node_name]
    #   separator: ;
    #   regex: ^(.*)$
    #   target_label: nodename
    #   replacement: $1
    #   action: replace

## Configuration for prometheus-node-exporter subchart
##
prometheus-node-exporter:
  podLabels:
    ## Add the 'node-exporter' label to be used by serviceMonitor to match standard common usage in rules and grafana dashboards
    ##
    jobLabel: node-exporter
  extraArgs:
    - --collector.filesystem.ignored-mount-points=^/(dev|proc|sys|var/lib/docker/.+)($|/)
    - --collector.filesystem.ignored-fs-types=^(autofs|binfmt_misc|cgroup|configfs|debugfs|devpts|devtmpfs|fusectl|hugetlbfs|mqueue|overlay|proc|procfs|pstore|rpc_pipefs|securityfs|sysfs|tracefs)$

## Manages Prometheus and Alertmanager components
##
prometheusOperator:
  enabled: true

  ## Service account for Alertmanager to use.
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
  ##
  serviceAccount:
    create: true
    name: ""

  ## Configuration for Prometheus operator service
  ##
  service:
    annotations: {}
    labels: {}
    clusterIP: ""

  ## Port to expose on each node
  ## Only used if service.type is 'NodePort'
  ##
    nodePort: 30080

  ## Additional ports to open for Prometheus service
  ## ref: https://kubernetes.io/docs/concepts/services-networking/service/#multi-port-services
  ##
    additionalPorts: []
    #  - name: thanos-cluster
    #    port: 10900
    #    nodePort: 30111

  ## Loadbalancer IP
  ## Only use if service.type is "loadbalancer"
  ##
    loadBalancerIP: ""
    loadBalancerSourceRanges: []

  ## Service type
  ## NodepPort, ClusterIP, loadbalancer
  ##
    type: ClusterIP

    ## List of IP addresses at which the Prometheus server service is available
    ## Ref: https://kubernetes.io/docs/user-guide/services/#external-ips
    ##
    externalIPs: []

  ## Deploy CRDs used by Prometheus Operator.
  ##
  createCustomResource: true

  ## Customize CRDs API Group
  crdApiGroup: monitoring.coreos.com

  ## Attempt to clean up CRDs created by Prometheus Operator.
  ##
  cleanupCustomResource: false

  ## Labels to add to the operator pod
  ##
  podLabels: {}

  ## Annotations to add to the operator pod
  ##
  podAnnotations: {}

  ## Assign a PriorityClassName to pods if set
  # priorityClassName: ""

  ## Define Log Format
  # Use logfmt (default) or json-formatted logging
  # logFormat: logfmt

  ## Decrease log verbosity to errors only
  # logLevel: error

  ## If true, the operator will create and maintain a service for scraping kubelets
  ## ref: https://github.com/coreos/prometheus-operator/blob/master/helm/prometheus-operator/README.md
  ##
  kubeletService:
    enabled: true
    namespace: kube-system

  ## Create a servicemonitor for the operator
  ##
  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    selfMonitor: true

  ## Resource limits & requests
  ##
  resources: {}
  # limits:
  #   cpu: 200m
  #   memory: 200Mi
  # requests:
  #   cpu: 100m
  #   memory: 100Mi

  ## Define which Nodes the Pods are scheduled on.
  ## ref: https://kubernetes.io/docs/user-guide/node-selection/
  ##
  nodeSelector: {}

  ## Tolerations for use with node taints
  ## ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
  ##
  tolerations: []
  # - key: "key"
  #   operator: "Equal"
  #   value: "value"
  #   effect: "NoSchedule"

  ## Assign the prometheus operator to run on specific nodes
  ## ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
  ##
  affinity: {}
  # requiredDuringSchedulingIgnoredDuringExecution:
  #   nodeSelectorTerms:
  #   - matchExpressions:
  #     - key: kubernetes.io/e2e-az-name
  #       operator: In
  #       values:
  #       - e2e-az1
  #       - e2e-az2

  securityContext:
    runAsNonRoot: true
    runAsUser: 65534

  ## Prometheus-operator image
  ##
  image:
    repository: quay.io/coreos/prometheus-operator
    tag: v0.29.0
    pullPolicy: IfNotPresent

  ## Configmap-reload image to use for reloading configmaps
  ##
  configmapReloadImage:
    repository: quay.io/coreos/configmap-reload
    tag: v0.0.1

  ## Prometheus-config-reloader image to use for config and rule reloading
  ##
  prometheusConfigReloaderImage:
    repository: quay.io/coreos/prometheus-config-reloader
    tag: v0.29.0

  ## Set the prometheus config reloader side-car CPU limit. If unset, uses the prometheus-operator project default
  ##
  # configReloaderCpu: 100m

  ## Set the prometheus config reloader side-car memory limit. If unset, uses the prometheus-operator project default
  ##
  # configReloaderMemory: 25Mi

  ## Hyperkube image to use when cleaning up
  ##
  hyperkubeImage:
    repository: k8s.gcr.io/hyperkube
    tag: v1.12.1
    pullPolicy: IfNotPresent

## Deploy a Prometheus instance
##
prometheus:

  enabled: true

  ## Service account for Prometheuses to use.
  ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
  ##
  serviceAccount:
    create: true
    name: ""

  ## Configuration for Prometheus service
  ##
  service:
    annotations: {}
    labels: {}
    clusterIP: ""


    ## To be used with a proxy extraContainer port
    targetPort: 9090

    ## List of IP addresses at which the Prometheus server service is available
    ## Ref: https://kubernetes.io/docs/user-guide/services/#external-ips
    ##
    externalIPs: []

    ## Port to expose on each node
    ## Only used if service.type is 'NodePort'
    ##
    nodePort: 30090

    ## Loadbalancer IP
    ## Only use if service.type is "loadbalancer"
    loadBalancerIP: ""
    loadBalancerSourceRanges: []
    ## Service type
    ##
    type: ClusterIP

    sessionAffinity: ""

  rbac:
    ## Create role bindings in the specified namespaces, to allow Prometheus monitoring
    ## a role binding in the release namespace will always be created.
    ##
    roleNamespaces:
      - kube-system

  ## Configure pod disruption budgets for Prometheus
  ## ref: https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget
  ## This configuration is immutable once created and will require the PDB to be deleted to be changed
  ## https://github.com/kubernetes/kubernetes/issues/45398
  ##
  podDisruptionBudget:
    enabled: false
    minAvailable: 1
    maxUnavailable: ""

  ingress:
    enabled: false
    annotations: {}
    labels: {}

    ## Hostnames.
    ## Must be provided if Ingress is enabled.
    ##
    # hosts:
    #   - prometheus.domain.com
    hosts: []

    ## Paths to use for ingress rules - one path should match the prometheusSpec.routePrefix
    ##
    paths: []
    # - /

    ## TLS configuration for Prometheus Ingress
    ## Secret must be manually created in the namespace
    ##
    tls: []
      # - secretName: prometheus-general-tls
      #   hosts:
      #     - prometheus.example.com

  serviceMonitor:
    ## Scrape interval. If not set, the Prometheus default scrape interval is used.
    ##
    interval: ""
    selfMonitor: true

  ## Settings affecting prometheusSpec
  ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheusspec
  ##
  prometheusSpec:

    ## Interval between consecutive scrapes.
    ##
    scrapeInterval: ""

    ## Interval between consecutive evaluations.
    ##
    evaluationInterval: ""

    ## ListenLocal makes the Prometheus server listen on loopback, so that it does not bind against the Pod IP.
    ##
    listenLocal: false

    ## Image of Prometheus.
    ##
    image:
      repository: quay.io/prometheus/prometheus
      tag: v2.7.2

    ## Tolerations for use with node taints
    ## ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/
    ##
    tolerations: []
    #  - key: "key"
    #    operator: "Equal"
    #    value: "value"
    #    effect: "NoSchedule"

    ## Alertmanagers to which alerts will be sent
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#alertmanagerendpoints
    ##
    ## Default configuration will connect to the alertmanager deployed as part of this release
    ##
    alertingEndpoints: []
    # - name: ""
    #   namespace: ""
    #   port: http
    #   scheme: http

    ## External labels to add to any time series or alerts when communicating with external systems
    ##
    externalLabels: {}

    ## External URL at which Prometheus will be reachable.
    ##
    externalUrl: ""

    ## Define which Nodes the Pods are scheduled on.
    ## ref: https://kubernetes.io/docs/user-guide/node-selection/
    ##
    nodeSelector: {}

    ## Secrets is a list of Secrets in the same namespace as the Prometheus object, which shall be mounted into the Prometheus Pods.
    ## The Secrets are mounted into /etc/prometheus/secrets/. Secrets changes after initial creation of a Prometheus object are not
    ## reflected in the running Pods. To change the secrets mounted into the Prometheus Pods, the object must be deleted and recreated
    ## with the new list of secrets.
    ##
    secrets: []

    ## ConfigMaps is a list of ConfigMaps in the same namespace as the Prometheus object, which shall be mounted into the Prometheus Pods.
    ## The ConfigMaps are mounted into /etc/prometheus/configmaps/.
    ##
    configMaps: []

    ## QuerySpec defines the query command line flags when starting Prometheus.
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#queryspec
    ##
    query: {}

    ## Namespaces to be selected for PrometheusRules discovery.
    ## If nil, select own namespace. Namespaces to be selected for ServiceMonitor discovery.
    ## See https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#namespaceselector for usage
    ##
    ruleNamespaceSelector: {}

    ## If true, a nil or {} value for prometheus.prometheusSpec.ruleSelector will cause the
    ## prometheus resource to be created with selectors based on values in the helm deployment,
    ## which will also match the PrometheusRule resources created
    ##
    ruleSelectorNilUsesHelmValues: true

    ## PrometheusRules to be selected for target discovery.
    ## If {}, select all ServiceMonitors
    ##
    ruleSelector: {}
    ## Example which select all prometheusrules resources
    ## with label "prometheus" with values any of "example-rules" or "example-rules-2"
    # ruleSelector:
    #   matchExpressions:
    #     - key: prometheus
    #       operator: In
    #       values:
    #         - example-rules
    #         - example-rules-2
    #
    ## Example which select all prometheusrules resources with label "role" set to "example-rules"
    # ruleSelector:
    #   matchLabels:
    #     role: example-rules

    ## If true, a nil or {} value for prometheus.prometheusSpec.serviceMonitorSelector will cause the
    ## prometheus resource to be created with selectors based on values in the helm deployment,
    ## which will also match the servicemonitors created
    ##
    serviceMonitorSelectorNilUsesHelmValues: true

    ## ServiceMonitors to be selected for target discovery.
    ## If {}, select all ServiceMonitors
    ##
    serviceMonitorSelector: {}
    ## Example which selects ServiceMonitors with label "prometheus" set to "somelabel"
    # serviceMonitorSelector:
    #   matchLabels:
    #     prometheus: somelabel

    ## Namespaces to be selected for ServiceMonitor discovery.
    ## See https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#namespaceselector for usage
    ##
    serviceMonitorNamespaceSelector: {}

    ## How long to retain metrics
    ##
    retention: 10d

    ## If true, the Operator won't process any Prometheus configuration changes
    ##
    paused: false

    ## Number of Prometheus replicas desired
    ##
    replicas: 1

    ## Log level for Prometheus be configured in
    ##
    logLevel: info

    ## Prefix used to register routes, overriding externalUrl route.
    ## Useful for proxies that rewrite URLs.
    ##
    routePrefix: /

    ## Standard object’s metadata. More info: https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
    ## Metadata Labels and Annotations gets propagated to the prometheus pods.
    ##
    podMetadata: {}
    # labels:
    #   app: prometheus
    #   k8s-app: prometheus

    ## Pod anti-affinity can prevent the scheduler from placing Prometheus replicas on the same node.
    ## The default value "soft" means that the scheduler should *prefer* to not schedule two replica pods onto the same node but no guarantee is provided.
    ## The value "hard" means that the scheduler is *required* to not schedule two replica pods onto the same node.
    ## The value "" will disable pod anti-affinity so that no anti-affinity rules will be configured.
    podAntiAffinity: ""

    ## If anti-affinity is enabled sets the topologyKey to use for anti-affinity.
    ## This can be changed to, for example, failure-domain.beta.kubernetes.io/zone
    ##
    podAntiAffinityTopologyKey: kubernetes.io/hostname

    ## The remote_read spec configuration for Prometheus.
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#remotereadspec
    remoteRead: []
    # - url: http://remote1/read

    ## The remote_write spec configuration for Prometheus.
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#remotewritespec
    remoteWrite: []
    # - url: http://remote1/push

    ## Resource limits & requests
    ##
    resources: {}
    # requests:
    #   memory: 400Mi

    ## Prometheus StorageSpec for persistent data
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/user-guides/storage.md
    ##
    storageSpec: {}
    #  volumeClaimTemplate:
    #    spec:
    #      storageClassName: gluster
    #      accessModes: ["ReadWriteOnce"]
    #      resources:
    #        requests:
    #          storage: 50Gi
    #    selector: {}

    ## AdditionalScrapeConfigs allows specifying additional Prometheus scrape configurations. Scrape configurations
    ## are appended to the configurations generated by the Prometheus Operator. Job configurations must have the form
    ## as specified in the official Prometheus documentation:
    ## https://prometheus.io/docs/prometheus/latest/configuration/configuration/#<scrape_config>. As scrape configs are
    ## appended, the user is responsible to make sure it is valid. Note that using this feature may expose the possibility
    ## to break upgrades of Prometheus. It is advised to review Prometheus release notes to ensure that no incompatible
    ## scrape configs are going to break Prometheus after the upgrade.
    ##
    ## The scrape configuraiton example below will find master nodes, provided they have the name .*mst.*, relabel the
    ## port to 2379 and allow etcd scraping provided it is running on all Kubernetes master nodes
    ##
    additionalScrapeConfigs: []
    # - job_name: kube-etcd
    #   kubernetes_sd_configs:
    #     - role: node
    #   scheme: https
    #   tls_config:
    #     ca_file:   /etc/prometheus/secrets/etcd-client-cert/etcd-ca
    #     cert_file: /etc/prometheus/secrets/etcd-client-cert/etcd-client
    #     key_file:  /etc/prometheus/secrets/etcd-client-cert/etcd-client-key
    #   relabel_configs:
    #   - action: labelmap
    #     regex: __meta_kubernetes_node_label_(.+)
    #   - source_labels: [__address__]
    #     action: replace
    #     target_label: __address__
    #     regex: ([^:;]+):(\d+)
    #     replacement: ${1}:2379
    #   - source_labels: [__meta_kubernetes_node_name]
    #     action: keep
    #     regex: .*mst.*
    #   - source_labels: [__meta_kubernetes_node_name]
    #     action: replace
    #     target_label: node
    #     regex: (.*)
    #     replacement: ${1}
    #   metric_relabel_configs:
    #   - regex: (kubernetes_io_hostname|failure_domain_beta_kubernetes_io_region|beta_kubernetes_io_os|beta_kubernetes_io_arch|beta_kubernetes_io_instance_type|failure_domain_beta_kubernetes_io_zone)
    #     action: labeldrop


    ## AdditionalAlertManagerConfigs allows for manual configuration of alertmanager jobs in the form as specified
    ## in the official Prometheus documentation https://prometheus.io/docs/prometheus/latest/configuration/configuration/#<alertmanager_config>.
    ## AlertManager configurations specified are appended to the configurations generated by the Prometheus Operator.
    ## As AlertManager configs are appended, the user is responsible to make sure it is valid. Note that using this
    ## feature may expose the possibility to break upgrades of Prometheus. It is advised to review Prometheus release
    ## notes to ensure that no incompatible AlertManager configs are going to break Prometheus after the upgrade.
    ##
    additionalAlertManagerConfigs: []
    # - consul_sd_configs:
    #   - server: consul.dev.test:8500
    #     scheme: http
    #     datacenter: dev
    #     tag_separator: ','
    #     services:
    #       - metrics-prometheus-alertmanager

    ## AdditionalAlertRelabelConfigs allows specifying Prometheus alert relabel configurations. Alert relabel configurations specified are appended
    ## to the configurations generated by the Prometheus Operator. Alert relabel configurations specified must have the form as specified in the
    ## official Prometheus documentation: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#alert_relabel_configs.
    ## As alert relabel configs are appended, the user is responsible to make sure it is valid. Note that using this feature may expose the
    ## possibility to break upgrades of Prometheus. It is advised to review Prometheus release notes to ensure that no incompatible alert relabel
    ## configs are going to break Prometheus after the upgrade.
    ##
    additionalAlertRelabelConfigs: []
    # - separator: ;
    #   regex: prometheus_replica
    #   replacement: $1
    #   action: labeldrop

    ## SecurityContext holds pod-level security attributes and common container settings.
    ## This defaults to non root user with uid 1000 and gid 2000.
    ## https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md
    ##
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 2000

    ## 	Priority class assigned to the Pods
    ##
    priorityClassName: ""

    ## Thanos configuration allows configuring various aspects of a Prometheus server in a Thanos environment.
    ## This section is experimental, it may change significantly without deprecation notice in any release.
    ## This is experimental and may change significantly without backward compatibility in any release.
    ## ref: https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#thanosspec
    ##
    thanos: {}

    ## Containers allows injecting additional containers. This is meant to allow adding an authentication proxy to a Prometheus pod.
    ##  if using proxy extraContainer  update targetPort with proxy container port
    containers: []

    ## Enable additional scrape configs that are managed externally to this chart. Note that the prometheus
    ## will fail to provision if the correct secret does not exist.
    ##
    additionalScrapeConfigsExternal: false

  additionalServiceMonitors: []
  ## Name of the ServiceMonitor to create
  ##
  # - name: ""

    ## Additional labels to set used for the ServiceMonitorSelector. Together with standard labels from
    ## the chart
    ##
    # additionalLabels: {}

    ## Service label for use in assembling a job name of the form <label value>-<port>
    ## If no label is specified, the service name is used.
    ##
    # jobLabel: ""

    ## Label selector for services to which this ServiceMonitor applies
    ##
    # selector: {}

    ## Namespaces from which services are selected
    ##
    # namespaceSelector:
      ## Match any namespace
      ##
      # any: false

      ## Explicit list of namespace names to select
      ##
      # matchNames: []

    ## Endpoints of the selected service to be monitored
    ##
    # endpoints: []
      ## Name of the endpoint's service port
      ## Mutually exclusive with targetPort
      # - port: ""

      ## Name or number of the endpoint's target port
      ## Mutually exclusive with port
      # - targetPort: ""

      ## File containing bearer token to be used when scraping targets
      ##
      #   bearerTokenFile: ""

      ## Interval at which metrics should be scraped
      ##
      #   interval: 30s

      ## HTTP path to scrape for metrics
      ##
      #   path: /metrics

      ## HTTP scheme to use for scraping
      ##
      #   scheme: http

      ## TLS configuration to use when scraping the endpoint
      ##
      #   tlsConfig:

          ## Path to the CA file
          ##
          # caFile: ""

          ## Path to client certificate file
          ##
          # certFile: ""

          ## Skip certificate verification
          ##
          # insecureSkipVerify: false

          ## Path to client key file
          ##
          # keyFile: ""

          ## Server name used to verify host name
          ##
          # serverName: ""
`
