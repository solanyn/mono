package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var xComputeNodeGVR = schema.GroupVersionResource{Group: "home-ops.io", Version: "v1alpha1", Resource: "xcomputenodes"}
var xComputeNodeGVK = schema.GroupVersionKind{Group: "home-ops.io", Version: "v1alpha1", Kind: "XComputeNode"}

type BurstReconciler struct {
	client.Client
	MaxNodes    int
	IdleTimeout time.Duration
	Namespace   string
}

func (r *BurstReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	if err := r.scaleUp(ctx); err != nil {
		logger.Error(err, "scale-up check failed")
	}
	if err := r.scaleDown(ctx); err != nil {
		logger.Error(err, "scale-down check failed")
	}

	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *BurstReconciler) scaleUp(ctx context.Context) error {
	logger := log.FromContext(ctx)

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"status.phase": "Pending"}); err != nil {
		return err
	}

	pending := 0
	for _, p := range pods.Items {
		if wantsBurst(&p) {
			pending++
		}
	}
	if pending == 0 {
		return nil
	}

	existing, err := r.listXComputeNodes(ctx)
	if err != nil {
		return err
	}
	count := len(existing.Items)

	toCreate := pending
	if count+toCreate > r.MaxNodes {
		toCreate = r.MaxNodes - count
	}

	for i := 0; i < toCreate; i++ {
		name := fmt.Sprintf("burst-%s", randSuffix())
		if err := r.createXComputeNode(ctx, name); err != nil {
			return err
		}
		logger.Info("created XComputeNode", "name", name)
	}
	return nil
}

func (r *BurstReconciler) scaleDown(ctx context.Context) error {
	logger := log.FromContext(ctx)

	var nodes corev1.NodeList
	if err := r.List(ctx, &nodes, client.MatchingLabels{"node-role.kubernetes.io/burst": "true"}); err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if !isNodeReady(&node) {
			continue
		}
		idle, since := r.isNodeIdle(ctx, &node)
		if !idle || time.Since(since) < r.IdleTimeout {
			continue
		}

		logger.Info("scaling down idle burst node", "node", node.Name)

		patch := client.MergeFrom(node.DeepCopy())
		node.Spec.Unschedulable = true
		if err := r.Patch(ctx, &node, patch); err != nil {
			return err
		}

		xcn := &unstructured.Unstructured{}
		xcn.SetGroupVersionKind(xComputeNodeGVK)
		xcn.SetName(node.Name)
		xcn.SetNamespace(r.Namespace)
		if err := r.Delete(ctx, xcn); client.IgnoreNotFound(err) != nil {
			return err
		}
		logger.Info("deleted XComputeNode", "name", node.Name)
	}
	return nil
}

func (r *BurstReconciler) listXComputeNodes(ctx context.Context) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "home-ops.io", Version: "v1alpha1", Kind: "XComputeNodeList"})
	err := r.List(ctx, list, client.InNamespace(r.Namespace))
	return list, err
}

func (r *BurstReconciler) createXComputeNode(ctx context.Context, name string) error {
	var cm corev1.ConfigMap
	if err := r.Get(ctx, types.NamespacedName{Name: "burst-config", Namespace: r.Namespace}, &cm); err != nil {
		return fmt.Errorf("reading burst-config: %w", err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(xComputeNodeGVK)
	obj.SetName(name)
	obj.SetNamespace(r.Namespace)

	spec := map[string]interface{}{}
	for _, k := range []string{"zone", "machineType", "spot", "diskSizeGb", "image", "userData"} {
		if v, ok := cm.Data[k]; ok {
			spec[k] = v
		}
	}
	obj.Object["spec"] = spec

	return r.Create(ctx, obj)
}

func (r *BurstReconciler) isNodeIdle(ctx context.Context, node *corev1.Node) (bool, time.Time) {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return false, time.Time{}
	}
	for _, p := range pods.Items {
		if !isDaemonSetPod(&p) {
			return false, time.Time{}
		}
	}
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return true, c.LastTransitionTime.Time
		}
	}
	return true, time.Now().Add(-24 * time.Hour)
}

func wantsBurst(p *corev1.Pod) bool {
	if p.Spec.NodeSelector != nil {
		if v, ok := p.Spec.NodeSelector["burst"]; ok && v == "true" {
			return true
		}
	}
	if p.Spec.Affinity != nil && p.Spec.Affinity.NodeAffinity != nil &&
		p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		for _, term := range p.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, expr := range term.MatchExpressions {
				if expr.Key == "node-role.kubernetes.io/burst" {
					return true
				}
			}
		}
	}
	return false
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func isDaemonSetPod(p *corev1.Pod) bool {
	for _, ref := range p.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func randSuffix() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func (r *BurstReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "status.phase", func(o client.Object) []string {
		return []string{string(o.(*corev1.Pod).Status.Phase)}
	}); err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.nodeName", func(o client.Object) []string {
		return []string{o.(*corev1.Pod).Spec.NodeName}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
