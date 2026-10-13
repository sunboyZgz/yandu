package storage

import (
	"net/http"
	"strings"
	"yandu/internal/storageapi"
)

type contractDelegate struct{ s *Service }

func (s *Service) PublicHandler() http.Handler {
	generated := storageapi.Handler(contractDelegate{s})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/storage/v1/monitor" {
			s.serveInventory(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/tus/") {
			s.ServeHTTP(w, r)
			return
		}
		generated.ServeHTTP(w, r)
	})
}
func (d contractDelegate) GetCapabilities(w http.ResponseWriter, r *http.Request, projectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) GetCapacity(w http.ResponseWriter, r *http.Request, projectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) DownloadDerivative(w http.ResponseWriter, r *http.Request, projectId string, derivativeId string, params storageapi.DownloadDerivativeParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) DownloadDerivativeHead(w http.ResponseWriter, r *http.Request, projectId string, derivativeId string, params storageapi.DownloadDerivativeHeadParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListEntries(w http.ResponseWriter, r *http.Request, projectId string, params storageapi.ListEntriesParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) DeleteEntry(w http.ResponseWriter, r *http.Request, projectId string, entryId string, params storageapi.DeleteEntryParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) GetEntry(w http.ResponseWriter, r *http.Request, projectId string, entryId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) MoveEntry(w http.ResponseWriter, r *http.Request, projectId string, entryId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) CreateFolder(w http.ResponseWriter, r *http.Request, projectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListJobs(w http.ResponseWriter, r *http.Request, projectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ProcessObject(w http.ResponseWriter, r *http.Request, projectId string, params storageapi.ProcessObjectParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) DownloadObject(w http.ResponseWriter, r *http.Request, projectId string, objectId string, params storageapi.DownloadObjectParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) DownloadObjectHead(w http.ResponseWriter, r *http.Request, projectId string, objectId string, params storageapi.DownloadObjectHeadParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListVersions(w http.ResponseWriter, r *http.Request, projectId string, objectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListOperations(w http.ResponseWriter, r *http.Request, projectId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListTrash(w http.ResponseWriter, r *http.Request, projectId string, params storageapi.ListTrashParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) RestoreEntry(w http.ResponseWriter, r *http.Request, projectId string, entryId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) CreateUploadIntent(w http.ResponseWriter, r *http.Request, projectId string, params storageapi.CreateUploadIntentParams) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) GetUploadIntent(w http.ResponseWriter, r *http.Request, projectId string, uploadId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) ListDerivatives(w http.ResponseWriter, r *http.Request, projectId string, objectId string) {
	d.s.ServeHTTP(w, r)
}

func (d contractDelegate) GetJob(w http.ResponseWriter, r *http.Request, projectId, jobId string) {
	d.s.ServeHTTP(w, r)
}
func (d contractDelegate) CancelJob(w http.ResponseWriter, r *http.Request, projectId, jobId string) {
	d.s.ServeHTTP(w, r)
}
