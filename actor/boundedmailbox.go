package actor

import (
	"runtime"
	"sync/atomic"

	"github.com/Workiva/go-datastructures/queue"
)

type BoundedMailbox struct {
	throughput      int
	userMailbox     *queue.RingBuffer
	systemMailbox   *queue.RingBuffer
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
}

func (mailbox *BoundedMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Put(message)
	mailbox.schedule()
}

func (mailbox *BoundedMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *BoundedMailbox) Suspend() {

}

func (mailbox *BoundedMailbox) Resume() {

}

func (mailbox *BoundedMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)

	done := false
	for !done {
		//process x messages in sequence, then exit
		for i := 0; i < mailbox.throughput; i++ {
			if mailbox.systemMailbox.Len() > 0 {
				sysMsg, _ := mailbox.systemMailbox.Get()
				sys, _ := sysMsg.(SystemMessage)
				mailbox.systemInvoke(sys)
			} else if mailbox.userMailbox.Len() > 0 {
				userMsg, _ := mailbox.userMailbox.Get()
				mailbox.userInvoke(userMsg)
			} else {
				done = true
				break
			}
		}
		runtime.Gosched()
	}

	//set mailbox to idle
	atomic.StoreInt32(&mailbox.schedulerStatus, MailboxIdle)
	//check if there are still messages to process (sent after the message loop ended)
	if atomic.SwapInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages) == MailboxHasMoreMessages {
		mailbox.schedule()
	}

}

func NewBoundedMailbox(throughput int, size int) MailboxProducer {
	return func() Mailbox {
		userMailbox := queue.NewRingBuffer(uint64(size))
		systemMailbox := queue.NewRingBuffer(100)
		mailbox := BoundedMailbox{
			throughput:      throughput,
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: MailboxHasNoMessages,
			schedulerStatus: MailboxIdle,
		}
		return &mailbox
	}
}

func (mailbox *BoundedMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
