package main

import (
	"Mini-Redis/resp"
	"Mini-Redis/tcp"
	"fmt"
)

func main() {
	fmt.Println("Mini-Redis Server 启动中....")
	handler := &resp.RespHandler{}
	server := tcp.NewServer(":6379", handler)
	fmt.Println("开始监听....")
	server.ListenAndServe()
}

type ListNode struct {
	Val  int
	Next *ListNode
}

func removeNthFromEnd(head *ListNode, n int) *ListNode {
	// 采取哨兵
	fakeNode := &ListNode{
		Val:  0,
		Next: head,
	}
	// 能找到倒数第k个节点
	p, q, curr := fakeNode, fakeNode, fakeNode
	for i := n; i > 0; i-- {
		q = q.Next
	}
	for ; q != nil; q = q.Next {
		curr = p
		p = p.Next
	}
	curr.Next = p.Next
	return fakeNode.Next
}

//func getIntersectionNode(headA, headB *ListNode) *ListNode {
//	//分别获取A和B的长度
//	lenA, lenB := 0, 0
//	p, q := headA, headB
//	for ; p != nil; p = p.Next {
//		lenA++
//	}
//	for ; q != nil; q = q.Next {
//		lenB++
//	}
//
//	if lenA-lenB > 0 {
//		//A先走step
//		for i := 0; i < lenA-lenB; i++ {
//			headA = headA.Next
//		}
//		for ; headA != nil && headB != nil; headA, headB = headA.Next, headB.Next {
//			if headA == headB {
//				return headA
//			}
//		}
//		return nil
//	} else if lenB-lenA > 0 {
//		//A先走step
//		for i := 0; i < lenB-lenA; i++ {
//			headB = headB.Next
//		}
//		for ; headA != nil && headB != nil; headA, headB = headA.Next, headB.Next {
//			if headA == headB {
//				return headA
//			}
//		}
//		return nil
//	}
//
//	for ; headA != nil && headB != nil; headA, headB = headA.Next, headB.Next {
//		if headA == headB {
//			return headA
//		}
//	}
//	return nil
//}

// 构造相等路径

func getIntersectionNode(headA, headB *ListNode) *ListNode {
	p, q := headA, headB
	for p != q {
		if p != nil {
			p = p.Next
		} else {
			p = headB
		}
		if q != nil {
			q = q.Next
		} else {
			q = headA
		}
	}
	return p
}

// 快慢指针
func detectCycle(head *ListNode) *ListNode {

	return nil
}
