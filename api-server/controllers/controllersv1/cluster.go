package controllersv1

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/bentoml/yatai-schemas/modelschemas"
	"github.com/bentoml/yatai-schemas/schemasv1"
	"github.com/bentoml/yatai/api-server/models"
	"github.com/bentoml/yatai/api-server/services"
	"github.com/bentoml/yatai/api-server/transformers/transformersv1"
	"github.com/bentoml/yatai/common/utils"
)

type clusterController struct {
	baseController
}

var ClusterController = clusterController{}

type GetClusterSchema struct {
	GetOrganizationSchema
	ClusterName string `path:"clusterName"`
}

func (s *GetClusterSchema) GetCluster(ctx context.Context) (*models.Cluster, error) {
	org, err := s.GetOrganization(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "get organization %s", s.OrgName)
	}
	cluster, err := services.ClusterService.GetByName(ctx, org.ID, s.ClusterName)
	if err != nil {
		return nil, errors.Wrapf(err, "get cluster %s", s.ClusterName)
	}
	return cluster, nil
}

func (c *clusterController) canView(ctx context.Context, cluster *models.Cluster) error {
	user, err := services.GetCurrentUser(ctx)
	if err != nil {
		return err
	}
	return services.MemberService.CanView(ctx, &services.ClusterMemberService, user, cluster.ID)
}

func (c *clusterController) canUpdate(ctx context.Context, cluster *models.Cluster) error {
	user, err := services.GetCurrentUser(ctx)
	if err != nil {
		return err
	}
	return services.MemberService.CanUpdate(ctx, &services.ClusterMemberService, user, cluster.ID)
}

func (c *clusterController) canOperate(ctx context.Context, cluster *models.Cluster) error {
	user, err := services.GetCurrentUser(ctx)
	if err != nil {
		return err
	}
	return services.MemberService.CanOperate(ctx, &services.ClusterMemberService, user, cluster.ID)
}

type CreateClusterSchema struct {
	schemasv1.CreateClusterSchema
	GetOrganizationSchema
}

func (c *clusterController) Create(ctx *gin.Context, schema *CreateClusterSchema) (*schemasv1.ClusterFullSchema, error) {
	user, err := services.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	org, err := schema.GetOrganization(ctx)
	if err != nil {
		return nil, err
	}

	if err = OrganizationController.canOperate(ctx, org); err != nil {
		return nil, err
	}

	cluster, createError := services.ClusterService.Create(ctx, services.CreateClusterOption{
		CreatorId:      user.ID,
		OrganizationId: org.ID,
		Name:           schema.Name,
		Description:    schema.Description,
		KubeConfig:     schema.KubeConfig,
		Config:         schema.Config,
	})

	apiTokenName := ""
	if user.ApiToken != nil {
		apiTokenName = user.ApiToken.Name
	}
	createEventOpt := services.CreateEventOption{
		CreatorId:      user.ID,
		ApiTokenName:   apiTokenName,
		OrganizationId: &org.ID,
		ResourceType:   modelschemas.ResourceTypeCluster,
		ResourceId:     cluster.ID,
		Status:         modelschemas.EventStatusSuccess,
		OperationName:  "created",
	}
	if createError != nil {
		createEventOpt.Status = modelschemas.EventStatusFailed
	}
	if _, eventError := services.EventService.Create(ctx, createEventOpt); eventError != nil {
		return nil, errors.Wrap(eventError, "create event")
	}
	if createError != nil {
		return nil, errors.Wrap(err, "create cluster")
	}
	return transformersv1.ToClusterFullSchema(ctx, cluster)
}

type UpdateClusterSchema struct {
	schemasv1.UpdateClusterSchema
	GetClusterSchema
}

func (c *clusterController) Update(ctx *gin.Context, schema *UpdateClusterSchema) (*schemasv1.ClusterFullSchema, error) {
	cluster, err := schema.GetCluster(ctx)
	if err != nil {
		return nil, err
	}
	if err = c.canUpdate(ctx, cluster); err != nil {
		return nil, err
	}
	cluster, updateError := services.ClusterService.Update(ctx, cluster, services.UpdateClusterOption{
		Description: schema.Description,
		Config:      schema.Config,
		KubeConfig:  schema.KubeConfig,
	})
	user, err := services.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	org, err := schema.GetOrganization(ctx)
	if err != nil {
		return nil, err
	}
	apiTokenName := ""
	if user.ApiToken != nil {
		apiTokenName = user.ApiToken.Name
	}
	createEventOpt := services.CreateEventOption{
		CreatorId:      user.ID,
		ApiTokenName:   apiTokenName,
		OrganizationId: &org.ID,
		ResourceType:   modelschemas.ResourceTypeCluster,
		ResourceId:     cluster.ID,
		Status:         modelschemas.EventStatusSuccess,
		OperationName:  "updated",
	}
	if updateError != nil {
		createEventOpt.Status = modelschemas.EventStatusFailed
	}
	if _, eventError := services.EventService.Create(ctx, createEventOpt); eventError != nil {
		return nil, errors.Wrap(eventError, "create event")
	}
	if updateError != nil {
		return nil, errors.Wrap(err, "update cluster")
	}
	return transformersv1.ToClusterFullSchema(ctx, cluster)
}

func (c *clusterController) Get(ctx *gin.Context, schema *GetClusterSchema) (*schemasv1.ClusterFullSchema, error) {
	cluster, err := schema.GetCluster(ctx)
	if err != nil {
		return nil, err
	}
	if err = c.canView(ctx, cluster); err != nil {
		return nil, err
	}
	return transformersv1.ToClusterFullSchema(ctx, cluster)
}

type ListClusterSchema struct {
	schemasv1.ListQuerySchema
	GetOrganizationSchema
}

func (c *clusterController) List(ctx *gin.Context, schema *ListClusterSchema) (*schemasv1.ClusterListSchema, error) {
	org, err := schema.GetOrganization(ctx)
	if err != nil {
		return nil, err
	}

	if err = OrganizationController.canView(ctx, org); err != nil {
		return nil, err
	}

	clusters, total, err := services.ClusterService.List(ctx, services.ListClusterOption{
		BaseListOption: services.BaseListOption{
			Start:  utils.UintPtr(schema.Start),
			Count:  utils.UintPtr(schema.Count),
			Search: schema.Search,
		},
		OrganizationId: utils.UintPtr(org.ID),
	})
	if err != nil {
		return nil, errors.Wrap(err, "list clusters")
	}

	clusterSchemas, err := transformersv1.ToClusterSchemas(ctx, clusters)
	return &schemasv1.ClusterListSchema{
		BaseListSchema: schemasv1.BaseListSchema{
			Total: total,
			Start: schema.Start,
			Count: schema.Count,
		},
		Items: clusterSchemas,
	}, err
}

func (c *clusterController) GetDockerRegistryRef(ctx *gin.Context, schema *GetClusterSchema) (*modelschemas.DockerRegistryRefSchema, error) {
	cluster, err := schema.GetCluster(ctx)
	if err != nil {
		return nil, err
	}
	if err = c.canView(ctx, cluster); err != nil {
		return nil, err
	}
	return services.ClusterService.GetDockerRegistryRef(ctx, cluster)
}

func (c *clusterController) WsPods(ctx *gin.Context, schema *GetClusterSchema) (err error) {
	ctx.Request.Header.Del("Origin")
	conn, err := wsUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logrus.Errorf("ws connect failed: %q", err.Error())
		return
	}
	defer conn.Close()

	cluster, err := schema.GetCluster(ctx)
	if err != nil {
		return
	}
	if err = c.canView(ctx, cluster); err != nil {
		return
	}

	defer func() {
		if err != nil {
			msg := schemasv1.WsRespSchema{
				Type:    schemasv1.WsRespTypeError,
				Message: err.Error(),
				Payload: make([]*schemasv1.KubePodSchema, 0),
			}
			_ = conn.WriteJSON(&msg)
		}
	}()

	namespace := ctx.Query("namespace")
	selectors_ := strings.Split(ctx.Query("selector"), ";")
	selectors := make([]labels.Selector, 0, len(selectors_))
	for _, selector_ := range selectors_ {
		selector, err := labels.Parse(selector_)
		if err != nil {
			return errors.Wrap(err, "parse selector")
		}
		selectors = append(selectors, selector)
	}

	podInformer, podLister, err := services.GetPodInformer(ctx, cluster, namespace)
	if err != nil {
		return
	}

	pollingCtx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			mt, _, err := conn.ReadMessage()

			if err != nil || mt == websocket.CloseMessage || mt == -1 {
				cancel()
				break
			}
		}
	}()

	failedCount := atomic.NewInt64(0)
	maxFailed := int64(10)

	fail := func() {
		failedCount.Inc()
	}

	send := func() {
		var err error
		defer func() {
			if err != nil {
				fail()
			}
		}()
		pods := make([]*models.KubePodWithStatus, 0)
		for _, selector := range selectors {
			pods_, err := services.KubePodService.ListPodsBySelector(ctx, cluster, namespace, podLister, selector)
			if err != nil {
				return
			}
			pods = append(pods, pods_...)
		}
		podSchemas, err := transformersv1.ToKubePodSchemas(ctx, cluster.ID, pods)
		if err != nil {
			return
		}
		err = conn.WriteJSON(schemasv1.WsRespSchema{
			Type:    schemasv1.WsRespTypeSuccess,
			Message: "",
			Payload: podSchemas,
		})
	}

	send()

	informer := podInformer.Informer()
	defer runtime.HandleCrash()

	checkPod := func(obj interface{}) bool {
		pod, ok := obj.(*apiv1.Pod)
		if !ok {
			return false
		}
		for _, selector := range selectors {
			if selector.Matches(labels.Set(pod.Labels)) {
				return true
			}
		}
		return false
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if !checkPod(obj) {
				return
			}
			send()
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if !checkPod(newObj) {
				return
			}
			send()
		},
		DeleteFunc: func(obj interface{}) {
			if !checkPod(obj) {
				return
			}
			send()
		},
	})

	func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()

		for {
			select {
			case <-pollingCtx.Done():
				return
			default:
			}

			if failedCount.Load() > maxFailed {
				logrus.Error("ws pods failed too frequently!")
				break
			}

			<-ticker.C
		}
	}()

	return
}
