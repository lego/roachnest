package cluster

type ClusterSettings struct {
	Size int
}

type Cluster interface {
	Cleanup() error
}
