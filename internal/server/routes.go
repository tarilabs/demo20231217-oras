package server

import (
	"demo20231217-oras/internal/restoras"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	oras "oras.land/oras-go/v2"
	orasfile "oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type MyRESTORASImpl struct{}

type FormTType1 struct {
	FileName *multipart.FileHeader `form:"fileName"`
	Var1     string                `form:"var1"`
}

func (*MyRESTORASImpl) PostUpload(c *gin.Context) {
	var req FormTType1
	if err := c.ShouldBind(&req); err != nil {
		fmt.Printf("error: %v", err)
	}
	fmt.Println(req)
	file, err := req.FileName.Open()
	checkErr(err)
	data, err := io.ReadAll(file)
	checkErr(err)
	// fmt.Println("Data: ", string(data))

	dname, err := os.MkdirTemp("", "sampledir")
	checkErr(err)
	fmt.Println("dname for temporary local storage (might in-memory later): ", dname)

	f, err := os.CreateTemp(dname, req.FileName.Filename)
	checkErr(err)
	fmt.Println("file for temporary local storage (might in-memory later): ", f)
	_, err = f.Write(data)
	checkErr(err)
	err = f.Close()
	checkErr(err)

	fs, err := orasfile.New(dname)
	checkErr(err)
	defer fs.Close()

	mediaType := "application/vnd.test.file"
	fileNames := []string{f.Name()}
	fileDescriptors := make([]v1.Descriptor, 0, len(fileNames))
	for _, name := range fileNames {
		fileDescriptor, err := fs.Add(c.Request.Context(), name, mediaType, "")
		checkErr(err)
		fileDescriptors = append(fileDescriptors, fileDescriptor)
		fmt.Printf("file descriptor for %s: %v\n", name, fileDescriptor)
	}

	artifactType := "application/vnd.test.artifact"
	opts := oras.PackManifestOptions{
		Layers: fileDescriptors,
	}
	manifestDescriptor, err := oras.PackManifest(c.Request.Context(), fs, oras.PackManifestVersion1_1_RC4, artifactType, opts)
	checkErr(err)
	fmt.Println("manifest descriptor:", manifestDescriptor)

	tag := req.Var1
	if err = fs.Tag(c.Request.Context(), manifestDescriptor, tag); err != nil {
		fmt.Println("Error: ", err)
	}

	reg := os.Getenv("REGISTRY")
	repo, err := remote.NewRepository(reg + "/mmortari/orastest")
	checkErr(err)
	repo.Client = &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.DefaultCache,
		Credential: auth.StaticCredential(reg, auth.Credential{
			Username: os.Getenv("USERNAME"),
			Password: os.Getenv("PASSWORD"),
		}),
	}

	_, err = oras.Copy(c.Request.Context(), fs, tag, repo, tag, oras.DefaultCopyOptions)
	checkErr(err)

	resp := make(map[string]string)
	resp["message"] = "Hello World"
	c.JSON(http.StatusOK, resp)
}

func checkErr(e error) {
	if e != nil {
		fmt.Printf("error: %v", e)
	}
}

func (s *Server) RegisterRoutes() http.Handler {
	myApi := &MyRESTORASImpl{}
	r := gin.Default()

	r.GET("/", s.HelloWorldHandler)
	restoras.RegisterHandlers(r, myApi)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler, ginSwagger.URL("../openapi.yaml")))
	r.StaticFile("openapi.yaml", "./api/openapi.yaml")

	return r
}

func (s *Server) HelloWorldHandler(c *gin.Context) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	c.JSON(http.StatusOK, resp)
}
