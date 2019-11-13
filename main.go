package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var cache = make(map[string]map[string]interface{})

type Secret struct {
	Key  string `json:"key,omitempty" yaml:"key,omitempty"`
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

type Plugin struct {
	metav1.TypeMeta
	metav1.ObjectMeta     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Data                  map[string]Secret `json:"data,omitempty" yaml:"data,omitempty"`
	Type                  corev1.SecretType `json:"type,omitempty" yaml:"type,omitempty"`
	Behavior              string            `json:"behavior,omitempty" yaml:"behavior,omitempty"`
	DisableNameSuffixHash bool              `json:"disableNameSuffixHash,omitempty" yaml:"disableNameSuffixHash,omitempty"`
}

func NewPlugin() Plugin {
	p := Plugin{}
	p.DisableNameSuffixHash = false
	p.Behavior = "create"
	p.Type = "Opaque"
	return p
}

func (p *Plugin) Read(fileName string) error {
	b, err := ioutil.ReadFile(os.Args[1])

	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, p)
}

func (p *Plugin) GenerateSecret() corev1.Secret {
	s := corev1.Secret{}
	s.SetGroupVersionKind(schema.FromAPIVersionAndKind("v1", "Secret"))
	s.SetName(p.GetName())
	s.SetNamespace(p.GetNamespace())
	s.SetLabels(p.GetLabels())
	s.Type = p.Type
	s.Data = make(map[string][]byte)

	a := make(map[string]string)
	d := map[string]string{
		"kustomize.config.k8s.io/needs-hash": strconv.FormatBool(!p.DisableNameSuffixHash),
		"kustomize.config.k8s.io/behavior":   p.Behavior,
	}

	for k, v := range p.GetAnnotations() {
		a[k] = v
	}

	for k, v := range d {
		a[k] = v
	}

	s.SetAnnotations(a)

	return s
}

func (p *Plugin) HydrateSecret(s *corev1.Secret) error {
	sess := session.New()
	svc := secretsmanager.New(sess)

	for k, v := range p.Data {
		sv, err := GetSecret(svc, v.Name)

		if err != nil {
			return err
		}

		s.Data[k] = []byte(sv[v.Key].(string))
	}

	return nil
}

func GetSecret(svc *secretsmanager.SecretsManager, id string) (map[string]interface{}, error) {
	if val, ok := cache[id]; ok {
		return val, nil
	}

	i := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(id),
	}

	o, err := svc.GetSecretValue(i)

	if err != nil {
		return nil, err
	}

	s := make(map[string]interface{})

	err = json.Unmarshal([]byte(*o.SecretString), &s)

	if err != nil {
		return nil, err
	}

	cache[id] = s

	return cache[id], nil
}

func main() {
	p := NewPlugin()
	err := p.Read(os.Args[1])

	if err != nil {
		panic(err)
	}

	s := p.GenerateSecret()
	p.HydrateSecret(&s)

	b, err := yaml.Marshal(s)

	if err != nil {
		panic(err)
	}

	fmt.Print(string(b))
}
