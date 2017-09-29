package calcium

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/archive"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// FIXME in alpine, useradd rename as adduser
const (
	fromTmpl   = "FROM %s"
	fromAsTmpl = "FROM %s as %s"
	commonTmpl = `ENV ERU 1
{{ if .Source }}ADD {{.Repo}} {{.Home}}/{{.Name}}
{{ else }}RUN mkdir -p {{.Home}}/{{.Name}}{{ end }}
WORKDIR {{.Home}}/{{.Name}}
RUN useradd -u {{.UID}} -d /nonexistent -s /sbin/nologin -U {{.User}}
RUN chown -R {{.UID}} {{.Home}}/{{.Name}}`
	runTmpl  = "RUN sh -c \"%s\""
	copyTmpl = "COPY --from=%s %s %s"
	userTmpl = "USER %s"
)

// Get a random node from pod `podname`
func getRandomNode(c *calcium, podname string) (*types.Node, error) {
	nodes, err := c.ListPodNodes(podname, false)
	if err != nil {
		log.Errorf("[getRandomNode] Error during ListPodNodes for %s: %v", podname, err)
		return nil, err
	}
	if len(nodes) == 0 {
		err = fmt.Errorf("No nodes available in pod %s", podname)
		log.Errorf("[getRandomNode] Error during getRandomNode from %s: %v", podname, err)
		return nil, err
	}

	nodemap := make(map[string]types.CPUMap)
	for _, n := range nodes {
		nodemap[n.Name] = n.CPU
	}
	nodename, err := c.scheduler.RandomNode(nodemap)
	if err != nil {
		log.Errorf("[getRandomNode] Error during getRandomNode from %s: %v", podname, err)
		return nil, err
	}
	if nodename == "" {
		err = fmt.Errorf("Got empty node during getRandomNode from %s", podname)
		return nil, err
	}

	return c.GetNode(podname, nodename)
}

// BuildImage will build image for repository
// since we wanna set UID for the user inside container, we have to know the uid parameter
//
// build directory is like:
//
//    buildDir ├─ :appname ├─ code
//             ├─ Dockerfile
func (c *calcium) BuildImage(opts *types.BuildOptions) (chan *types.BuildImageMessage, error) {
	ch := make(chan *types.BuildImageMessage)

	// get pod from config
	buildPodname := c.config.Docker.BuildPod
	if buildPodname == "" {
		return ch, fmt.Errorf("No build pod set in config")
	}

	// get node by scheduler
	node, err := getRandomNode(c, buildPodname)
	if err != nil {
		return ch, err
	}

	// make build dir
	buildDir, err := ioutil.TempDir(os.TempDir(), "corebuild-")
	if err != nil {
		return ch, err
	}
	defer os.RemoveAll(buildDir)

	initBuild := opts.Builds.Builds[opts.Builds.Stages[0]]

	// parse repository name
	// code locates under /:repositoryname
	reponame, err := utils.GetGitRepoName(initBuild.Repo)
	if err != nil {
		return ch, err
	}

	// clone code into cloneDir
	// which is under buildDir and named as repository name
	cloneDir := filepath.Join(buildDir, reponame)
	if err := c.source.SourceCode(initBuild.Repo, cloneDir, initBuild.Version); err != nil {
		return ch, err
	}

	// ensure source code is safe
	// we don't want any history files to be retrieved
	if err := c.source.Security(cloneDir); err != nil {
		return ch, err
	}

	// if artifact download url is provided, remove all source code to
	// improve security
	if len(initBuild.Artifacts) > 0 {
		os.RemoveAll(cloneDir)
		os.MkdirAll(cloneDir, os.ModeDir)
		for _, artifact := range initBuild.Artifacts {
			if err := c.source.Artifact(artifact, cloneDir); err != nil {
				return ch, err
			}
		}
	}

	// create dockerfile
	// rs := richSpecs{specs, uid, strings.TrimRight(c.config.AppDir, "/"), reponame, true}
	if opts.Home == "" {
		opts.Home = c.config.AppDir
	}
	if opts.UID == 0 {
		return ch, errors.New("Need user id")
	}
	if len(opts.Builds.Stages) == 1 {
		if err := makeSimpleDockerFile(opts, initBuild, buildDir); err != nil {
			return ch, err
		}
	} else {
		if err := makeComplexDockerFile(opts, buildDir); err != nil {
			return ch, err
		}
	}

	// tag of image, later this will be used to push image to hub
	tag := createImageTag(c.config.Docker, opts.Name, opts.Tag)

	// create tar stream for Build API
	buildContext, err := createTarStream(buildDir)
	if err != nil {
		return ch, err
	}

	// must be put here because of that `defer os.RemoveAll(buildDir)`
	buildOptions := enginetypes.ImageBuildOptions{
		Tags:           []string{tag},
		SuppressOutput: false,
		NoCache:        true,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
	}

	log.Infof("[BuildImage] Building image %v at %v:%v", tag, buildPodname, node.Name)
	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalTimeout)
	defer cancel()
	resp, err := node.Engine.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return ch, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		decoder := json.NewDecoder(resp.Body)
		for {
			message := &types.BuildImageMessage{}
			err := decoder.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Errorf("[BuildImage] Decode build image message failed %v", err)
				return
			}
			ch <- message
		}

		// About this "Khadgar", https://github.com/docker/docker/issues/10983#issuecomment-85892396
		// Just because Ben Schnetzer's cute Khadgar...
		rc, err := node.Engine.ImagePush(context.Background(), tag, enginetypes.ImagePushOptions{RegistryAuth: "Khadgar"})
		if err != nil {
			ch <- makeErrorBuildImageMessage(err)
			return
		}

		defer rc.Close()
		decoder2 := json.NewDecoder(rc)
		for {
			message := &types.BuildImageMessage{}
			err := decoder2.Decode(message)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Errorf("[BuildImage] Decode push image message failed %v", err)
				return
			}
			ch <- message
		}

		// 无论如何都删掉build机器的
		// 事实上他不会跟cached pod一样
		// 一样就砍死
		go func() {
			_, err := node.Engine.ImageRemove(context.Background(), tag, enginetypes.ImageRemoveOptions{
				Force:         false,
				PruneChildren: true,
			})
			if err != nil {
				log.Errorf("[BuildImage] Remove image error: %s", err)
			}
		}()

		ch <- &types.BuildImageMessage{Status: "finished", Progress: tag}
	}()

	return ch, nil
}

func makeErrorBuildImageMessage(err error) *types.BuildImageMessage {
	return &types.BuildImageMessage{Error: err.Error()}
}

func createTarStream(path string) (io.ReadCloser, error) {
	tarOpts := &archive.TarOptions{
		ExcludePatterns: []string{},
		IncludeFiles:    []string{"."},
		Compression:     archive.Uncompressed,
		NoLchown:        true,
	}
	return archive.TarWithOptions(path, tarOpts)
}

func makeCommonPart(opts *types.BuildOptions, build *types.Build) (string, error) {
	tmpl := template.Must(template.New("dockerfile").Parse(commonTmpl))
	out := bytes.Buffer{}
	if err := tmpl.Execute(&out,
		struct {
			*types.BuildOptions
			*types.Build
		}{opts, build}); err != nil {
		return "", err
	}
	return out.String(), nil
}

func makeMainPart(opts *types.BuildOptions, build *types.Build, from, commands string, copys []string) (string, error) {
	var buildTmpl []string
	common, err := makeCommonPart(opts, build)
	if err != nil {
		return "", err
	}
	buildTmpl = append(buildTmpl, from, common)
	if len(copys) > 0 {
		buildTmpl = append(buildTmpl, copys...)
	}
	buildTmpl = append(buildTmpl, commands, "")
	return strings.Join(buildTmpl, "\n"), nil
}

func makeSimpleDockerFile(opts *types.BuildOptions, build *types.Build, buildDir string) error {
	from := fmt.Sprintf(fromTmpl, build.Base)
	user := ""
	if opts.User != "" {
		user = fmt.Sprintf(userTmpl, opts.User)
	}
	commands := fmt.Sprintf(runTmpl, strings.Join(build.Commands, " && "))
	build.Source = true
	mainPart, err := makeMainPart(opts, build, from, commands, []string{})
	if err != nil {
		return err
	}
	dockerfile := fmt.Sprintf("%s\n%s", mainPart, user)
	return createDockerfile(dockerfile, buildDir)
}

func makeComplexDockerFile(opts *types.BuildOptions, buildDir string) error {
	var preArtifacts map[string]string
	var preStage string
	var buildTmpl []string

	for _, stage := range opts.Builds.Stages {
		build, ok := opts.Builds.Builds[stage]
		if !ok {
			log.Warnf("[makeComplexDockerFile] Builds stage %s not defined", stage)
			continue
		}

		from := fmt.Sprintf(fromAsTmpl, build.Base, stage)
		copys := []string{}
		for src, dst := range preArtifacts {
			copys = append(copys, fmt.Sprintf(copyTmpl, preStage, src, dst))
		}
		commands := fmt.Sprintf(runTmpl, strings.Join(build.Commands, " && "))
		// decide add source or not
		mainPart, err := makeMainPart(opts, build, from, commands, copys)
		if err != nil {
			return err
		}
		buildTmpl = append(buildTmpl, mainPart)
		preStage = stage
		preArtifacts = build.Artifacts
	}
	buildTmpl = append(buildTmpl, fmt.Sprintf(userTmpl, opts.User))
	dockerfile := strings.Join(buildTmpl, "\n")
	return createDockerfile(dockerfile, buildDir)
}

// Dockerfile
func createDockerfile(dockerfile, buildDir string) error {
	f, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(dockerfile)
	return err
}

// Image tag
// 格式严格按照 Hub/HubPrefix/appname:version 来
func createImageTag(config types.DockerConfig, appname, version string) string {
	prefix := strings.Trim(config.Namespace, "/")
	if prefix == "" {
		return fmt.Sprintf("%s/%s:%s", config.Hub, appname, version)
	}
	return fmt.Sprintf("%s/%s/%s:%s", config.Hub, prefix, appname, version)
}
