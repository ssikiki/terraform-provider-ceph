package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"terraform-provider-ceph/ceph/helper/mutexkv"
	"terraform-provider-ceph/ceph/helper/utils"

	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"github.com/sirupsen/logrus"
)

//func ParseCephImg(img string) (poolName string, imgName string, snapName string, err error) {
//	//($pool/)?$name@$snap
//	re := regexp.MustCompile(`(?m)(\S*/)?(\S+)@(\S+)`)
//	groups := re.FindStringSubmatch(img)
//	if len(groups) != 4 {
//		return "", "", "", fmt.Errorf("ceph image name illegal: %s", img)
//	}
//	pool := strings.Trim(groups[1], "/")
//	return pool, groups[2], groups[3], nil
//}

func ParseCephVol(volPath string) (cluster string, poolName string, volumeName string, snapName string, err error) {
	path := strings.SplitN(volPath, "/", 3)
	if len(path) != 3 {
		return "", "", "", "", fmt.Errorf("ceph volume format illegal, need {cluster}/{pool}/{volume}")
	}
	tmp := strings.SplitN(path[2], "@", 2)
	if len(tmp) == 2 {
		return path[0], path[1], tmp[0], tmp[1], nil
	}
	return path[0], path[1], path[2], "", nil
}

type CephVolumeI interface {
	Close() error
	GetParent() (string, error)
	GetSize() (uint64, error)
	LookupSnapByName(name string) (CephSnapshotI, error)
	CreateSnapshot(name string) (CephSnapshotI, error)
	Flatten() error
}

type CephSnapshotI interface {
	Remove() error
	IsProtected() (bool, error)
	Protect() error
	Unprotect() error
	Rollback() error
}

type CephSnapshot struct {
	*rbd.Snapshot
	*rbd.Image
	Ioctx *rados.IOContext
}

func (s *CephSnapshot) Remove() error {
	_ = s.Snapshot.Unprotect()
	return s.Snapshot.Remove()
}

type CephVolume struct {
	cluster string
	*rbd.Image
	Ioctx *rados.IOContext
}

func (v *CephVolume) GetParent() (string, error) {
	parentPool := make([]byte, 128)
	parentName := make([]byte, 128)
	parentSnapname := make([]byte, 128)
	err := v.GetParentInfo(parentPool, parentName, parentSnapname)
	if err != nil {
		if strings.Contains(err.Error(), "No such file or directory") {
			return "", nil
		}
		return "", err
	}
	n := bytes.Index(parentPool, []byte{0})
	pPool := string(parentPool[:n])
	n = bytes.Index(parentName, []byte{0})
	pName := string(parentName[:n])
	n = bytes.Index(parentSnapname, []byte{0})
	pSnapname := string(parentSnapname[:n])
	return fmt.Sprintf("%s/%s/%s@%s", v.cluster, pPool, pName, pSnapname), nil
}

func (v *CephVolume) LookupSnapByName(name string) (CephSnapshotI, error) {
	snaps, err := v.Image.GetSnapshotNames()
	if err != nil {
		return nil, err
	}
	for _, snap := range snaps {
		if snap.Name == name {
			return &CephSnapshot{Snapshot: v.Image.GetSnapshot(name), Image: v.Image, Ioctx: v.Ioctx}, nil
		}
	}
	return nil, nil
}

func (v *CephVolume) CreateSnapshot(name string) (CephSnapshotI, error) {
	_ = v.Image.Flush()
	snapshot, err := v.Image.CreateSnapshot(name)
	if err != nil {
		return nil, err
	}
	return &CephSnapshot{Snapshot: snapshot, Image: v.Image, Ioctx: v.Ioctx}, nil
}

func (v *CephVolume) Flatten() error {
	_ = v.Image.Flush()
	return v.Image.Flatten()
}

func (v *CephVolume) Close() error {
	if v.Ioctx != nil {
		defer v.Ioctx.Destroy()
	}
	return v.Image.Close()
}

// CephClient ceph
type CephClient struct {
	*rados.Conn
	cluster string
	MutexKV *mutexkv.MutexKV
}

type monAttr struct {
	Addr string `json:"addr"`
}

type monMap struct {
	Mons []monAttr `json:"mons"`
}

// MonStat struct for output of ceph quorum_status
type MonStat struct {
	Monmap monMap `json:"monmap"`
}

type authUser struct {
	Entity string            `json:"entity"`
	Key    string            `json:"key"`
	Caps   map[string]string `json:"caps"`
}

// NewCephClient generate ceph client
func NewCephClient(cluster string) (*CephClient, error) {
	var err error
	cephClient, err := rados.NewConn()
	if err != nil {
		return nil, err
	}
	if cluster == "" {
		err = cephClient.ReadDefaultConfigFile()
	} else {
		err = cephClient.ReadConfigFile(fmt.Sprintf("/etc/ceph/%s.conf", cluster))
	}
	if err != nil {
		return nil, err
	}
	if err := cephClient.Connect(); err != nil {
		return nil, err
	}

	client := &CephClient{
		Conn:    cephClient,
		MutexKV: mutexkv.NewMutexKV(),
		cluster: cluster,
	}
	return client, nil
}

// Version of ceph
func (c *CephClient) Version() (string, error) {
	command, _ := json.Marshal(map[string]string{"prefix": "version"})
	buf, _, err := c.Conn.MonCommand(command)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// RadosVersion version of rados
func (c *CephClient) RadosVersion() string {
	major, minor, patch := rados.Version()
	return fmt.Sprintf("%v.%v.%v", major, minor, patch)
}

// RbdVersion version of rbd
func (c *CephClient) RbdVersion() string {
	major, minor, patch := rbd.Version()
	return fmt.Sprintf("%v.%v.%v", major, minor, patch)
}

// GetMons get ceph mons
func (c *CephClient) GetMons() (mons []string, err error) {
	prefix := fmt.Sprintf("quorum_status")
	logrus.Debugf("get ceph mons: ceph %s", prefix)
	command, _ := json.Marshal(map[string]string{"prefix": prefix})
	buf, _, err := c.Conn.MonCommand(command)
	if err != nil {
		return mons, err
	}
	var monStat MonStat
	if err := json.Unmarshal(buf, &monStat); err != nil {
		return mons, err
	}
	for _, i := range monStat.Monmap.Mons {
		mons = append(mons, strings.SplitN(i.Addr, "/", 2)[0])
	}
	return mons, nil
}

// InitClientUser init client user auth for pool
func (c *CephClient) InitClientUser(username string, pools ...string) (key string, err error) {
	c.MutexKV.Lock(c.cluster)
	defer c.MutexKV.Unlock(c.cluster)

	// try to add user
	prefix := fmt.Sprintf("auth get-or-create client.%s", username)
	logrus.Debugf("get ceph user: ceph %s", prefix)
	command, _ := json.Marshal(map[string]string{"prefix": prefix})
	buf, _, err := c.Conn.MonCommand(command)
	if err != nil {
		return "", err
	}
	var users []authUser
	if err := json.Unmarshal(buf, &users); err != nil {
		return "", err
	} else if len(users) == 0 {
		return "", nil
	}
	defer func() {
		if len(pools) == 0 {
			return
		}
		if _, ok := users[0].Caps["mon"]; !ok {
			users[0].Caps["mon"] = "allow r"
		}
		if _, ok := users[0].Caps["osd"]; !ok {
			users[0].Caps["osd"] = "allow class-read object_prefix rbd_children"
		}
		for _, pool := range pools {
			if strings.Contains(users[0].Caps["osd"], "pool="+pool) {
				continue
			}
			users[0].Caps["osd"] += fmt.Sprintf(", allow rwx pool=%s", pool)
		}
		var caps []string
		for k, v := range users[0].Caps {
			caps = append(caps, fmt.Sprintf("%s '%s'", k, v))
		}
		prefix = fmt.Sprintf("auth caps client.%s %s", username, strings.Join(caps, " "))
		logrus.Debugf("set ceph auth: ceph %s", prefix)
		command, _ = json.Marshal(map[string]string{"prefix": prefix})
		if _, _, err := c.Conn.MonCommand(command); err != nil {
			logrus.Errorf(err.Error())
		}
	}()
	return users[0].Key, nil
}

func (c *CephClient) CloneImg(basePool, baseName, baseSnap, pool, name string) (CephVolumeI, error) {
	ioctx, err := c.Conn.OpenIOContext(pool)
	if err != nil {
		return nil, fmt.Errorf("can't get ioctx of pool '%s': %v", pool, err)
	}
	// defer ioctx.Destroy()

	baseIoctx := ioctx
	if basePool == "" {
		basePool = pool
	}
	if basePool != pool {
		if baseIoctx, err = c.Conn.OpenIOContext(basePool); err != nil {
			return nil, fmt.Errorf("can't get ioctx of pool '%s': %v", basePool, err)
		}
		defer baseIoctx.Destroy()
	}

	vol, err := rbd.GetImage(baseIoctx, baseName).Clone(baseSnap, ioctx, name, 1, 22)
	if err != nil {
		return nil, fmt.Errorf("clone image '%s/%s@%s' failed: %v", basePool, baseName, baseSnap, err)
	}
	return &CephVolume{Image: vol, Ioctx: ioctx, cluster: c.cluster}, nil
}

func (c *CephClient) CreateVol(pool, name string, size uint64) (CephVolumeI, error) {
	ioctx, err := c.Conn.OpenIOContext(pool)
	if err != nil {
		return nil, fmt.Errorf("can't get ioctx of pool '%s': %v", pool, err)
	}
	// defer ioctx.Destroy()

	vol, err := rbd.Create(ioctx, name, size, 22)
	if err != nil {
		return nil, err
	}
	return &CephVolume{Image: vol, Ioctx: ioctx, cluster: c.cluster}, nil
}

func (c *CephClient) DeleteVol(pool, name string) error {
	volI, err := c.LookupVolByName(pool, name)
	if err != nil {
		return err
	} else if volI == nil {
		return nil
	}
	vol := volI.(*CephVolume)
	defer vol.Ioctx.Destroy()

	// removing should fail while image is opened
	_ = vol.Image.Close()
	return vol.Remove()
}

func (c *CephClient) DeleteSnap(pool, name, snapName string) error {
	vol, err := c.LookupVolByName(pool, name)
	if err != nil {
		return err
	} else if vol == nil {
		return nil
	}
	defer vol.Close()

	snap, err := vol.LookupSnapByName(snapName)
	if err != nil {
		return err
	} else if snap == nil {
		return nil
	}
	return snap.Remove()
}

func (c *CephClient) LookupVolByName(pool, name string) (CephVolumeI, error) {
	ioctx, err := c.Conn.OpenIOContext(pool)
	if err != nil {
		return nil, err
	}
	// defer ioctx.Destroy()

	vol, err := rbd.OpenImage(ioctx, name, rbd.NoSnapshot)
	if err == rbd.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &CephVolume{Image: vol, Ioctx: ioctx, cluster: c.cluster}, nil
}

func (c *CephClient) ExistPool(name string) (bool, error) {
	_, err := c.Conn.GetPoolByName(name)
	if err == rados.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (c *CephClient) CreatePool(name string) error {
	return c.Conn.MakePool(name)
}

func (c *CephClient) DeletePool(name string) error {
	ok, err := c.ExistPool(name)
	if err != nil {
		return err
	} else if !ok {
		return nil
	}

	if err = c.Conn.DeletePool(name); err == rados.ErrPermissionDenied {
		logrus.Warnf("storage pool '%s' delete failed: %v", name, err)
		return nil
	}
	return err
}

type StoragePoolInfo struct {
	Capacity   uint64
	Allocation uint64
	Available  uint64
	State      int
	StateDp    string
}

func (c *CephClient) GetInfo(poolName string) (ret *StoragePoolInfo, err error) {
	command, _ := json.Marshal(map[string]string{"prefix": "df", "format": "json"})
	buf, _, err := c.Conn.MonCommand(command)
	if err != nil {
		return nil, fmt.Errorf("storagepool %s get info failed: %v", poolName, err)
	}

	ret = &StoragePoolInfo{}
	var hasPool bool
	var percentUsed float64
	var pools []map[string]interface{}
	utils.GetValFromJson(buf, "pools").ToVal(&pools)
	for _, pool := range pools {
		if pool["name"] == poolName {
			stats := pool["stats"].(map[string]interface{})
			if tmp, ok := stats["percent_used"]; ok {
				percentUsed = tmp.(float64)
				if percentUsed > 1 {
					//ceph >= v12 版本存在percent_used字段
					//ceph v12 percent_used字段 > 1
					//ceph v15 percent_used字段 < 1 统一按<1处理
					percentUsed = percentUsed / 100
				}
			} else {
				percentUsed = stats["bytes_used"].(float64) / (stats["bytes_used"].(float64) + stats["max_avail"].(float64))
			}
			ret.Allocation = uint64(stats["bytes_used"].(float64))
			ret.Available = uint64(stats["max_avail"].(float64))
			ret.Capacity = uint64(stats["bytes_used"].(float64) / percentUsed)
			hasPool = true
			break
		}
	}
	if !hasPool {
		return nil, fmt.Errorf("storagepool %s doesn't exist", poolName)
	}

	command, _ = utils.JsonMarshal(map[string]string{"prefix": "status", "format": "json"})
	buf, _, err = c.Conn.MonCommand(command)
	if err == nil {
		ret.StateDp = utils.GetValFromJson(buf, "health", "status").ToString()
		if ret.StateDp == "" {
			ret.StateDp = utils.GetValFromJson(buf, "health", "overall_status").ToString()
		}
		if ret.StateDp == "HEALTH_OK" {
			ret.State = 2
		}
	}
	return ret, nil
}
