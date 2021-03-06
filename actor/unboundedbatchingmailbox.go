package actor

import "sync/atomic"

import "github.com/rogeralsing/gam/queue"
import _ "log"
import "runtime"

type UnboundedBatchingMailbox struct {
	userMailbox     *queue.Queue
	systemMailbox   *queue.Queue
	schedulerStatus int32
	hasMoreMessages int32
	userInvoke      func(interface{})
	systemInvoke    func(SystemMessage)
	batchSize       int
}

func (mailbox *UnboundedBatchingMailbox) PostUserMessage(message interface{}) {
	mailbox.userMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *UnboundedBatchingMailbox) PostSystemMessage(message SystemMessage) {
	mailbox.systemMailbox.Push(message)
	mailbox.schedule()
}

func (mailbox *UnboundedBatchingMailbox) schedule() {
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasMoreMessages) //we have more messages to process
	if atomic.CompareAndSwapInt32(&mailbox.schedulerStatus, MailboxIdle, MailboxRunning) {
		go mailbox.processMessages()
	}
}

func (mailbox *UnboundedBatchingMailbox) Suspend() {

}

func (mailbox *UnboundedBatchingMailbox) Resume() {

}

func (mailbox *UnboundedBatchingMailbox) processMessages() {
	//we are about to start processing messages, we can safely reset the message flag of the mailbox
	atomic.StoreInt32(&mailbox.hasMoreMessages, MailboxHasNoMessages)
	batchSize := mailbox.batchSize
	done := false

	for !done {
		if sysMsg, ok := mailbox.systemMailbox.Pop(); ok {

			first := sysMsg.(SystemMessage)
			mailbox.systemInvoke(first)
		} else if userMsg, ok := mailbox.userMailbox.PopMany(batchSize); ok {

			mailbox.userInvoke(userMsg)
		} else {
			done = true
			break
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

func NewUnboundedBatchingMailbox(batchSize int) MailboxProducer {

	return func() Mailbox {
		userMailbox := queue.New()
		systemMailbox := queue.New()
		mailbox := UnboundedBatchingMailbox{
			userMailbox:     userMailbox,
			systemMailbox:   systemMailbox,
			hasMoreMessages: MailboxHasNoMessages,
			schedulerStatus: MailboxIdle,
			batchSize:       batchSize,
		}
		return &mailbox
	}
}

func (mailbox *UnboundedBatchingMailbox) RegisterHandlers(userInvoke func(interface{}), systemInvoke func(SystemMessage)) {
	mailbox.userInvoke = userInvoke
	mailbox.systemInvoke = systemInvoke
}
