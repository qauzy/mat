package provider

import (
	"github.com/qauzy/mat/component/trie"
	C "github.com/qauzy/mat/constant"
	"github.com/qauzy/mat/log"
)

type domainStrategy struct {
	count      int
	domainTrie *trie.DomainTrie[struct{}]
	domainSet  *trie.DomainSet
}

func (d *domainStrategy) ShouldFindProcess() bool {
	return false
}

func (d *domainStrategy) Match(metadata *C.Metadata) bool {
	return d.domainSet != nil && d.domainSet.Has(metadata.RuleHost())
}

func (d *domainStrategy) Count() int {
	return d.count
}

func (d *domainStrategy) ShouldResolveIP() bool {
	return false
}

func (d *domainStrategy) Reset() {
	d.domainTrie = trie.New[struct{}]()
	d.domainSet = nil
	d.count = 0
}

func (d *domainStrategy) Insert(rule string) {
	err := d.domainTrie.Insert(rule, struct{}{})
	if err != nil {
		log.Warnln("invalid domain:[%s]", rule)
	} else {
		d.count++
	}
}

func (d *domainStrategy) FinishInsert() {
	d.domainSet = d.domainTrie.NewDomainSet()
	d.domainTrie = nil
}

func NewDomainStrategy() *domainStrategy {
	return &domainStrategy{}
}
