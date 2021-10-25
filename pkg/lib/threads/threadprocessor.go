package threads

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anytypeio/go-anytype-middleware/metrics"
	"github.com/textileio/go-threads/core/logstore"
	"github.com/textileio/go-threads/core/thread"
	threadsDb "github.com/textileio/go-threads/db"
	threadsUtil "github.com/textileio/go-threads/util"
)

type ThreadProcessor interface {
	Init(thread.ID, ...ThreadProcessorOption) error
	Listen(map[thread.ID]threadInfo) error
	GetThreadId() thread.ID
	GetThreadCollection() *threadsDb.Collection
	GetCollectionWithPrefix(prefix string) *threadsDb.Collection
	AddCollectionWithPrefix(prefix string, schema interface{}) (*threadsDb.Collection, error)
	GetDB() *threadsDb.DB
}

type CollectionActionProcessor func(action threadsDb.Action, collection *threadsDb.Collection)
type ThreadProcessorOption func(options *ThreadProcessorOptions)

type ThreadProcessorOptions struct {
	actionProcessors map[string]CollectionActionProcessor
	collections      map[string]interface{}
}

func NewThreadProcessorOptions() *ThreadProcessorOptions {
	return &ThreadProcessorOptions{
		actionProcessors: make(map[string]CollectionActionProcessor),
		collections:      make(map[string]interface{}),
	}
}

func WithCollection(collectionName string, schema interface{}) ThreadProcessorOption {
	return func(options *ThreadProcessorOptions) {
		options.collections[collectionName] = schema
	}
}

func WithCollectionAndActionProcessor(collectionName string,
	schema interface{},
	actionProcessor CollectionActionProcessor) ThreadProcessorOption {
	return func(options *ThreadProcessorOptions) {
		options.collections[collectionName] = schema
		options.actionProcessors[collectionName] = actionProcessor
	}
}

type threadProcessor struct {
	threadsService *service
	threadNotifier ThreadDownloadNotifier

	db                *threadsDb.DB
	threadsCollection *threadsDb.Collection
	collections       map[string]*threadsDb.Collection
	actionProcessors  map[string]CollectionActionProcessor

	isAccountProcessor bool

	threadId thread.ID
	sync.RWMutex
}

func (t *threadProcessor) GetCollectionWithPrefix(prefix string) *threadsDb.Collection {
	t.RLock()
	defer t.RUnlock()
	return t.collections[prefix]
}

func (t *threadProcessor) AddCollectionWithPrefix(prefix string, schema interface{}) (*threadsDb.Collection, error) {
	t.Lock()
	defer t.Unlock()
	fullName := prefix + t.threadId.String()
	coll, err := t.addCollection(fullName, schema)
	if err == nil {
		t.collections[prefix] = coll
	}
	return coll, err
}

func (t *threadProcessor) GetThreadCollection() *threadsDb.Collection {
	return t.threadsCollection
}

func (t *threadProcessor) GetDB() *threadsDb.DB {
	return t.db
}

func (t *threadProcessor) GetThreadId() thread.ID {
	return t.threadId
}

func NewThreadProcessor(s *service, notifier ThreadDownloadNotifier) ThreadProcessor {
	return &threadProcessor{
		threadsService: s,
		threadNotifier: notifier,
	}
}

func NewAccountThreadProcessor(s *service, simultaneousRequests int) ThreadProcessor {
	return &threadProcessor{
		threadsService:     s,
		isAccountProcessor: true,
		threadNotifier:     NewAccountNotifier(simultaneousRequests),
	}
}

func (t *threadProcessor) Init(id thread.ID, options ...ThreadProcessorOption) error {
	if t.db != nil {
		return nil
	}
	processorOptions := NewThreadProcessorOptions()
	for _, opt := range options {
		opt(processorOptions)
	}

	if id == thread.Undef {
		return fmt.Errorf("cannot start processor with undefined thread")
	}
	t.threadId = id

	tInfo, err := t.threadsService.t.GetThread(context.Background(), id)
	if err != nil {
		return fmt.Errorf("cannot start thread processor, because thread is not downloaded: %w", err)
	}

	t.db, err = threadsDb.NewDB(
		context.Background(),
		t.threadsService.threadsDbDS,
		t.threadsService.t,
		t.threadId,
		// We need to provide the key beforehand
		// otherwise there can be problems if the log is not created (and therefore the keys are not matched)
		// this happens with workspaces, because we are adding threads but not creating them
		threadsDb.WithNewKey(tInfo.Key),
		threadsDb.WithNewCollections())
	if err != nil {
		return err
	}

	threadIdString := t.threadId.String()
	// To not break the old behaviour we call account thread collection with the same name we used before
	if t.isAccountProcessor {
		threadIdString = ""
	}
	threadsCollectionName := ThreadInfoCollectionName + threadIdString
	t.collections = make(map[string]*threadsDb.Collection)

	t.threadsCollection, err = t.addCollection(threadsCollectionName, threadInfo{})
	if err != nil {
		return err
	}
	t.collections[ThreadInfoCollectionName] = t.threadsCollection

	for name, schema := range processorOptions.collections {
		_, err := t.AddCollectionWithPrefix(name, schema)
		if err != nil {
			return err
		}
	}
	t.actionProcessors = processorOptions.actionProcessors
	return nil
}

func (t *threadProcessor) Listen(initialThreads map[thread.ID]threadInfo) error {
	WorkspaceLogger.
		With("is account", t.isAccountProcessor).
		With("workspace id", t.threadId).
		Debug("started listening for workspace")

	log.With("thread id", t.threadId).
		Info("listen for workspace")
	l, err := t.db.Listen()
	if err != nil {
		return err
	}

	threadsTotal := len(initialThreads)
	initialThreadsLock := sync.RWMutex{}

	removeElement := func(tid thread.ID) {
		log.With("thread id", tid.String()).
			Debug("removing thread from processing")
		initialThreadsLock.RLock()
		_, isInitialThread := initialThreads[tid]
		initialThreadsLock.RUnlock()

		if isInitialThread {
			initialThreadsLock.Lock()
			delete(initialThreads, tid)
			threadsTotal--
			t.threadNotifier.SetTotalThreads(threadsTotal)
			initialThreadsLock.Unlock()
		}
	}

	removeThread := func(tid string) {
		err := t.threadsService.objectDeleter.DeleteObject(tid)
		if err != nil && err != logstore.ErrThreadNotFound {
			log.With("thread id", tid).
				Debugf("failed to delete thread")
		}
	}

	processThread := func(tid thread.ID, ti threadInfo) {
		log.With("thread id", tid.String()).
			Debugf("trying to process new thread")
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		info, err := t.threadsService.t.GetThread(ctx, tid)
		cancel()
		if err != nil && err != logstore.ErrThreadNotFound {
			log.With("thread", tid.String()).
				Errorf("error getting thread while processing: %v", err)
			removeElement(tid)
			return
		}
		if info.ID != thread.Undef {
			// our own event
			removeElement(tid)
			return
		}

		WorkspaceLogger.
			With("is account", t.isAccountProcessor).
			With("workspace id", t.threadId).
			With("thread id", tid.String()).
			Debug("processing thread")
		metrics.ExternalThreadReceivedCounter.Inc()
		go func() {
			if err := t.threadsService.processNewExternalThreadUntilSuccess(tid, ti); err != nil {
				log.With("thread", tid.String()).Error("processNewExternalThreadUntilSuccess failed: %t", err.Error())
				return
			}

			initialThreadsLock.RLock()
			if len(initialThreads) == 0 {
				initialThreadsLock.RUnlock()
			} else {
				_, isInitialThread := initialThreads[tid]
				initialThreadsLock.RUnlock()
				if isInitialThread {
					initialThreadsLock.Lock()

					delete(initialThreads, tid)
					t.threadNotifier.AddThread()
					if len(initialThreads) == 0 {
						t.threadNotifier.Finish()
					}

					initialThreadsLock.Unlock()
				}
			}
		}()
	}

	processThreadActions := func(actions []threadsDb.Action) {
		for _, action := range actions {
			if !strings.HasPrefix(action.Collection, ThreadInfoCollectionName) {
				collectionName := strings.Split(action.Collection, t.threadId.String())[0]
				t.RLock()
				collection, collectionExists := t.collections[collectionName]
				actionProcessor, actionExists := t.actionProcessors[collectionName]
				t.RUnlock()

				if collectionExists && actionExists {
					go actionProcessor(action, collection)
				}
				continue
			}
			if action.Type == threadsDb.ActionDelete {
				removeThread(action.ID.String())
				continue
			}
			if action.Type != threadsDb.ActionCreate {
				log.Errorf("failed to process workspace thread db %s(type %d)", action.ID.String(), action.Type)
				continue
			}

			instanceBytes, err := t.threadsCollection.FindByID(action.ID)
			if err != nil {
				log.Errorf("failed to find thread info for id %s: %v", action.ID.String(), err)
				continue
			}

			ti := threadInfo{}
			threadsUtil.InstanceFromJSON(instanceBytes, &ti)
			tid, err := thread.Decode(ti.ID.String())
			if err != nil {
				log.Errorf("failed to parse thread id %s: %v", ti.ID.String(), err)
				continue
			}
			initialThreadsLock.RLock()
			if len(initialThreads) != 0 {
				_, ok := initialThreads[tid]
				// if we are already downloading this thread as initial one
				if ok {
					initialThreadsLock.RUnlock()
					continue
				}
			}
			initialThreadsLock.RUnlock()
			processThread(tid, ti)
		}
	}

	if threadsTotal != 0 {
		log.With("thread count", threadsTotal).
			Info("pulling initial threads")

		if os.Getenv("ANYTYPE_RECOVERY_PROGRESS") == "1" {
			log.Info("adding progress bar")
			t.threadNotifier.Start(t.threadsService.process)
		}
		t.threadNotifier.SetTotalThreads(threadsTotal)

		initialMapCopy := make(map[thread.ID]threadInfo)
		for tid, ti := range initialThreads {
			initialMapCopy[tid] = ti
		}

		// processing all initial threads if any
		go func() {
			for tid, ti := range initialMapCopy {
				log.With("thread id", tid.String()).
					Debugf("going to process initial thread")
				processThread(tid, ti)
			}
		}()
	}

	go func() {
		defer l.Close()
		deadline := 1 * time.Second
		tmr := time.NewTimer(deadline)
		flushBuffer := make([]threadsDb.Action, 0, 100)
		timerRead := false

		processBuffer := func() {
			if len(flushBuffer) == 0 {
				return
			}
			buffCopy := make([]threadsDb.Action, len(flushBuffer))
			for index, action := range flushBuffer {
				buffCopy[index] = action
			}
			flushBuffer = flushBuffer[:0]
			go processThreadActions(buffCopy)
		}

		for {
			select {
			case <-t.threadsService.ctx.Done():
				processBuffer()
				return
			case _ = <-tmr.C:
				timerRead = true
				// we don't have new messages for at least deadline and we have something to flush
				processBuffer()

			case c := <-l.Channel():
				log.With("thread id", c.ID.String()).
					Debugf("received new thread through channel")
				// as per docs the timer should be stopped or expired with drained channel
				// to be reset
				if !tmr.Stop() && !timerRead {
					<-tmr.C
				}
				tmr.Reset(deadline)
				timerRead = false
				flushBuffer = append(flushBuffer, c)
			}
		}
	}()

	return nil
}

func (t *threadProcessor) addCollection(
	collectionName string,
	schema interface{}) (collection *threadsDb.Collection, err error) {
	collection = t.db.GetCollection(collectionName)

	if collection != nil {
		return
	}

	collectionConfig := threadsDb.CollectionConfig{
		Name:   collectionName,
		Schema: threadsUtil.SchemaFromInstance(schema, false),
	}
	collection, err = t.db.NewCollection(collectionConfig)
	return
}