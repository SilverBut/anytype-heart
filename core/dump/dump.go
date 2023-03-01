package dump

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/anytypeio/go-anytype-middleware/app"
	"github.com/anytypeio/go-anytype-middleware/core/block"
	"github.com/anytypeio/go-anytype-middleware/core/block/editor/smartblock"
	"github.com/anytypeio/go-anytype-middleware/core/wallet"
	"github.com/anytypeio/go-anytype-middleware/pb"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/bundle"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/core"
	smartblocktype "github.com/anytypeio/go-anytype-middleware/pkg/lib/core/smartblock"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/database"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/localstore/addr"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/localstore/objectstore"
	"github.com/anytypeio/go-anytype-middleware/pkg/lib/pb/model"
	"github.com/anytypeio/go-anytype-middleware/util/pbtypes"
)

const Name = "dump"

const profileFile = "profile"

type Service struct {
	objectStore  objectstore.ObjectStore
	blockService *block.Service
	app.Component
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Name() string {
	return Name
}

func (s *Service) Init(a *app.App) (err error) {
	s.objectStore = a.MustComponent(objectstore.CName).(objectstore.ObjectStore)
	s.blockService = a.MustComponent(block.CName).(*block.Service)
	return nil
}

func (s *Service) Dump(path string, mnemonic string, profile core.Profile, rootPath string) error {
	objectIDs, _, err := s.objectStore.QueryObjectIds(database.Query{}, nil)
	if err != nil {
		return fmt.Errorf("failed to QueryObjectIds: %v", err)
	}

	deletedObjects, _, err := s.objectStore.QueryObjectInfo(database.Query{
		Filters: []*model.BlockContentDataviewFilter{{
			RelationKey: bundle.RelationKeyIsDeleted.String(),
			Condition:   model.BlockContentDataviewFilter_Equal,
			Value:       pbtypes.Bool(true),
		},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to QueryObjectIds: %v", err)
	}
	archivedObjects, _, err := s.objectStore.QueryObjectInfo(database.Query{
		Filters: []*model.BlockContentDataviewFilter{{
			RelationKey: bundle.RelationKeyIsArchived.String(),
			Condition:   model.BlockContentDataviewFilter_Equal,
			Value:       pbtypes.Bool(true),
		},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to QueryObjectIds: %v", err)
	}
	fullPath := buildPath(path)
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %v", err)
	}
	zw := zip.NewWriter(f)
	defer zw.Close()
	defer func() {
		if err != nil {
			os.Remove(fullPath)
		}
	}()

	pr := &pb.Profile{
		Mnemonic: mnemonic,
		Name:     profile.Name,
		Avatar:   profile.IconImage,
		Address:  profile.AccountAddr,
	}
	wErr := s.writeSnapshotToFile(zw, profileFile, pr)
	if wErr != nil {
		return wErr
	}
	err = s.writeAccountDir(rootPath, profile, zw)
	if err != nil {
		return err
	}

	for _, object := range deletedObjects {
		mo, mErr := s.getMigrationObjectFromObjectInfo(object)
		if mErr != nil {
			return mErr
		}
		wErr := s.writeSnapshotToFile(zw, object.Id, mo)
		if wErr != nil {
			return wErr
		}
	}

	for _, object := range archivedObjects {
		mo, mErr := s.getMigrationObjectFromObjectInfo(object)
		if mErr != nil {
			return mErr
		}
		wErr := s.writeSnapshotToFile(zw, object.Id, mo)
		if wErr != nil {
			return wErr
		}
	}

	for _, id := range objectIDs {
		if err = s.blockService.Do(id, func(b smartblock.SmartBlock) error {
			details := b.CombinedDetails()
			if sourceObject := pbtypes.GetString(details, bundle.RelationKeySourceObject.String()); sourceObject != "" {
				if strings.HasPrefix(sourceObject, addr.BundledObjectTypeURLPrefix) ||
					strings.HasPrefix(sourceObject, addr.BundledRelationURLPrefix) {
					return nil
				}
			}
			sbType, sErr := smartblocktype.SmartBlockTypeFromID(b.RootId())
			if sErr != nil {
				return fmt.Errorf("failed SmartBlockTypeFromID: %v", err)
			}
			if skipObject(sbType) {
				return nil
			}
			mo, mErr := s.getMigrationObject(b)
			if mErr != nil {
				return mErr
			}
			wErr := s.writeSnapshotToFile(zw, id, mo)
			if wErr != nil {
				return wErr
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed blockService.Do: %v", err)
		}
	}
	return err
}

func (s *Service) writeAccountDir(rootPath string, profile core.Profile, zw *zip.Writer) error {
	return filepath.Walk(filepath.Join(rootPath, profile.AccountAddr), func(file string, fi os.FileInfo, err error) error {
		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		if strings.Contains(file, wallet.KeyFileDevice) || strings.Contains(file, wallet.KeyFileAccount) {
			header.Name = strings.TrimPrefix(filepath.ToSlash(file), rootPath)

			var wr io.Writer
			if wr, err = zw.CreateHeader(header); err != nil {
				return err
			}
			// if not a dir, write file content
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(wr, data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) getMigrationObjectFromObjectInfo(object *model.ObjectInfo) (*pb.MigrationObject, error) {
	sbType, err := smartblocktype.SmartBlockTypeFromID(object.Id)
	if err != nil {
		return nil, fmt.Errorf("failed SmartBlockTypeFromID: %v", err)
	}
	sn := &model.SmartBlockSnapshotBase{
		Details:     object.GetDetails(),
		ObjectTypes: object.GetObjectTypeUrls(),
	}
	mo := &pb.MigrationObject{
		SbType:   sbType.ToProto(),
		Snapshot: &pb.ChangeSnapshot{Data: sn},
	}
	return mo, nil
}

func (s *Service) getMigrationObject(b smartblock.SmartBlock) (*pb.MigrationObject, error) {
	st := b.NewState()
	rootID := st.RootId()
	sbType, err := smartblocktype.SmartBlockTypeFromID(rootID)
	if err != nil {
		return nil, fmt.Errorf("failed SmartBlockTypeFromID: %v", err)
	}
	removedCollectionKeys := make([]string, 0, len(st.StoreKeysRemoved()))

	sn := &model.SmartBlockSnapshotBase{
		Blocks:                st.Blocks(),
		Details:               st.CombinedDetails(),
		ObjectTypes:           st.ObjectTypes(),
		RelationLinks:         st.GetRelationLinks(),
		Collections:           st.Store(),
		RemovedCollectionKeys: removedCollectionKeys,
		ExtraRelations:        st.OldExtraRelations(),
	}

	stFileKeys := b.GetAndUnsetFileKeys()
	fileKeys := make([]*pb.ChangeFileKeys, 0, len(stFileKeys))
	for _, key := range stFileKeys {
		key := key
		fileKeys = append(fileKeys, &key)
	}
	mo := &pb.MigrationObject{
		SbType:   sbType.ToProto(),
		Snapshot: &pb.ChangeSnapshot{Data: sn, FileKeys: fileKeys},
	}
	return mo, nil
}

func (s *Service) writeSnapshotToFile(zw *zip.Writer, name string, ob proto.Marshaler) error {
	wr, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("failed create file with snapshot: %v", err)
	}
	data, err := ob.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}
	_, err = wr.Write(data)
	if err != nil {
		return fmt.Errorf("failed write snapshot to file: %v", err)
	}
	return nil
}

func buildPath(path string) string {
	var sb strings.Builder
	sb.WriteString(path)
	sb.WriteRune(filepath.Separator)
	sb.WriteString(Name)
	sb.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	sb.WriteString(".zip")
	return sb.String()
}

func skipObject(objectType smartblocktype.SmartBlockType) bool {
	return objectType == smartblocktype.SmartBlockTypeBundledObjectType ||
		objectType == smartblocktype.SmartBlockTypeBundledTemplate ||
		objectType == smartblocktype.SmartBlockTypeBundledRelation ||
		objectType == smartblocktype.SmartBlockTypeWorkspaceOld ||
		objectType == smartblocktype.SmartBlockTypeArchive ||
		objectType == smartblocktype.SmartBlockTypeHome ||
		objectType == smartblocktype.SmartblockTypeMarketplaceRelation ||
		objectType == smartblocktype.SmartblockTypeMarketplaceTemplate ||
		objectType == smartblocktype.SmartblockTypeMarketplaceType ||
		objectType == smartblocktype.SmartBlockTypeFile
}