// 本文件：linkedList 为列表命令维护双向链表，pushFront 和 popFront 在分片锁内修改节点。

package database

type node struct {
	val        []byte
	prev, next *node
}

// 列表随所属分片一起加锁，避免再引入一套锁顺序。
type linkedList struct {
	head, tail *node
	len        int
}

func newLinkedList() *linkedList { return &linkedList{} }

func (l *linkedList) size() int { return l.len }

// 写计划已经取得字节所有权，这里只维护链表，避免字符串和列表各自决定是否拷贝。
func (l *linkedList) pushFront(value []byte) {
	node := &node{val: value, next: l.head}
	if l.head == nil {
		l.tail = node
	} else {
		l.head.prev = node
	}
	l.head = node
	l.len++
}

// 摘下的值已不属于引擎，可以直接交给调用者；最后一个节点消失时也要清掉尾指针。
func (l *linkedList) popFront() ([]byte, bool) {
	if l.head == nil {
		return nil, false
	}
	node := l.head
	l.head = node.next
	if l.head == nil {
		l.tail = nil
	} else {
		l.head.prev = nil
	}
	node.next = nil
	l.len--
	return node.val, true
}
