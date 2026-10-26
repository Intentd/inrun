package bootstrap

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func applySA(ctx context.Context, cs kubernetes.Interface, e ClusterEntry, dryRun bool, log func(string)) error {
	name := SAName(e)
	ns := e.SANamespace
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: Labels()},
	}
	if dryRun {
		log(fmt.Sprintf("dry-run: ServiceAccount %s/%s", ns, name))
		return nil
	}
	_, err := cs.CoreV1().ServiceAccounts(ns).Create(ctx, sa, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		log(fmt.Sprintf("ServiceAccount %s/%s: already exists", ns, name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating ServiceAccount: %w", err)
	}
	log(fmt.Sprintf("ServiceAccount %s/%s: created", ns, name))
	return nil
}

func applyClusterRole(ctx context.Context, cs kubernetes.Interface, e ClusterEntry, rules []rbacv1.PolicyRule, dryRun bool, log func(string)) error {
	name := ClusterRoleName(e)
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: Labels()},
		Rules:      rules,
	}
	if dryRun {
		log(fmt.Sprintf("dry-run: ClusterRole %s", name))
		return nil
	}
	_, err := cs.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		existing, getErr := cs.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("fetching ClusterRole for update: %w", getErr)
		}
		existing.Rules = rules
		if _, updateErr := cs.RbacV1().ClusterRoles().Update(ctx, existing, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("updating ClusterRole: %w", updateErr)
		}
		log(fmt.Sprintf("ClusterRole %s: updated (rules reflect current blueprint)", name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating ClusterRole: %w", err)
	}
	log(fmt.Sprintf("ClusterRole %s: created", name))
	return nil
}

func applyClusterRoleBinding(ctx context.Context, cs kubernetes.Interface, e ClusterEntry, dryRun bool, log func(string)) error {
	name := CRBName(e)
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: Labels()},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     ClusterRoleName(e),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      SAName(e),
			Namespace: e.SANamespace,
		}},
	}
	if dryRun {
		log(fmt.Sprintf("dry-run: ClusterRoleBinding %s", name))
		return nil
	}
	_, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		log(fmt.Sprintf("ClusterRoleBinding %s: already exists", name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating ClusterRoleBinding: %w", err)
	}
	log(fmt.Sprintf("ClusterRoleBinding %s: created", name))
	return nil
}

func applyTokenSecret(ctx context.Context, cs kubernetes.Interface, e ClusterEntry, dryRun bool, log func(string)) (string, error) {
	tokenName := TokenSecretName(e)
	saName := SAName(e)
	ns := e.SANamespace
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tokenName,
			Namespace:   ns,
			Labels:      Labels(),
			Annotations: map[string]string{"kubernetes.io/service-account.name": saName},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	if dryRun {
		log(fmt.Sprintf("dry-run: Secret %s/%s (token)", ns, tokenName))
		return "", nil
	}

	if existing, err := cs.CoreV1().Secrets(ns).Get(ctx, tokenName, metav1.GetOptions{}); err == nil {
		if token := strings.TrimSpace(string(existing.Data["token"])); token != "" {
			log(fmt.Sprintf("Secret %s/%s: already exists — reusing token", ns, tokenName))
			return token, nil
		}
	}

	if _, err := cs.CoreV1().Secrets(ns).Create(ctx, s, metav1.CreateOptions{}); err != nil && !k8serrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("creating token Secret: %w", err)
	}

	token, err := waitForToken(ctx, cs, ns, tokenName, log)
	if err != nil {
		return "", err
	}
	log(fmt.Sprintf("Secret %s/%s: token ready", ns, tokenName))
	return token, nil
}

func applyCredentialSecret(ctx context.Context, cs kubernetes.Interface, secretName, namespace, token string, caData []byte, log func(string)) error {
	data := map[string][]byte{"token": []byte(token)}
	if len(caData) > 0 {
		data["ca.crt"] = caData
	}
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace, Labels: Labels()},
		Type:       corev1.SecretTypeOpaque,
		Data:       data,
	}
	_, err := cs.CoreV1().Secrets(namespace).Create(ctx, s, metav1.CreateOptions{})
	if k8serrors.IsAlreadyExists(err) {
		existing, getErr := cs.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("fetching credential Secret for update: %w", getErr)
		}
		existing.Data = data
		if _, updateErr := cs.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("updating credential Secret: %w", updateErr)
		}
		log(fmt.Sprintf("Secret %s/%s: updated", namespace, secretName))
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating credential Secret: %w", err)
	}
	log(fmt.Sprintf("Secret %s/%s: created", namespace, secretName))
	return nil
}
