// +build !confonly

package router

//go:generate errorgen

import (
	"context"
	"runtime"
	"sort"
	"sync"

	"v2ray.com/core"
	"v2ray.com/core/common"
	"v2ray.com/core/common/session"
	"v2ray.com/core/features/dns"
	"v2ray.com/core/features/outbound"
	"v2ray.com/core/features/routing"
)

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		r := new(Router)
		if err := core.RequireFeatures(ctx, func(d dns.Client, ohm outbound.Manager) error {
			return r.Init(config.(*Config), d, ohm)
		}); err != nil {
			return nil, err
		}
		return r, nil
	}))
}

// Router is an implementation of routing.Router.
type Router struct {
	sync.Mutex
	domainStrategy     Config_DomainStrategy
	rules              []*Rule
	balancers          map[string]*Balancer
	dns                dns.Client
	targettag2indexmap map[string]int
	index2targettag    map[int]string
}

func NewRouter() *Router {
	con := NewConditionChan()
	con.Add(NewInboundTagMatcher([]string{"asdf"}))
	con.Add(NewProtocolMatcher([]string{"tls"}))
	con.Add(NewUserMatcher([]string{"bge"}))
	return &Router{
		domainStrategy:     Config_AsIs,
		rules:              []*Rule{&Rule{Condition: con}},
		balancers:          map[string]*Balancer{},
		targettag2indexmap: map[string]int{},
		index2targettag:    map[int]string{},
	}
}
func Romvededuplicate(users []string) []string {
	sort.Strings(users)
	j := 0
	for i := 1; i < len(users); i++ {
		if users[j] == users[i] {
			continue
		}
		j++
		// preserve the original data
		// in[i], in[j] = in[j], in[i]
		// only set what is required
		users[j] = users[i]
	}
	return users[:j+1]
}
func (r *Router) AddUsers(targettag string, emails []string) {
	r.Lock()
	defer r.Unlock()
	if index, ok := r.targettag2indexmap[targettag]; ok {
		if conditioncan, ok := r.rules[index].Condition.(*ConditionChan); ok {
			for _, condition := range *conditioncan {
				if usermatcher, ok := condition.(*UserMatcher); ok {
					usermatcher.user = Romvededuplicate(append(usermatcher.user, emails...))
					break
				}
			}
		} else if usermatcher, ok := r.rules[index].Condition.(*UserMatcher); ok {
			usermatcher.user = Romvededuplicate(append(usermatcher.user, emails...))

		}
	} else {
		tagStartIndex := len(r.rules)
		r.targettag2indexmap[targettag] = tagStartIndex
		r.index2targettag[tagStartIndex] = targettag
		r.rules = append(r.rules, &Rule{Condition: NewUserMatcher(emails), Tag: targettag})
	}
	runtime.GC()
}

func (r *Router) RemoveUser(Users []string) {
	r.Lock()
	defer r.Unlock()
	removed_index := make([]int, 0, len(r.rules))
	for _, email := range Users {
		for _, rl := range r.rules {
			conditions, ok := rl.Condition.(*ConditionChan)
			if ok {
				for _, v := range *conditions {
					usermatcher, ok := v.(*UserMatcher)
					if ok {
						index := -1
						for i, e := range usermatcher.user {
							if e == email {
								index = i
								break
							}
						}
						if index != -1 {
							usermatcher.user = append(usermatcher.user[:index], usermatcher.user[index+1:]...)
						}
						break
					}
				}
			} else {
				if usermatcher, ok := rl.Condition.(*UserMatcher); ok {
					index := -1
					for i, e := range usermatcher.user {
						if e == email {
							index = i
							break
						}
					}
					if index != -1 {
						usermatcher.user = append(usermatcher.user[:index], usermatcher.user[index+1:]...)
					}
				}
			}

		}
	}
	for index, rl := range r.rules {
		conditions, ok := rl.Condition.(*ConditionChan)
		if ok {
			for _, v := range *conditions {
				usermatcher, ok := v.(*UserMatcher)
				if ok {
					if len(usermatcher.user) == 0 {
						removed_index = append(removed_index, index)
						break
					}

				}
			}
		} else {
			usermatcher, ok := rl.Condition.(*UserMatcher)
			if ok {
				if len(usermatcher.user) == 0 {
					removed_index = append(removed_index, index)
				}
			}
		}

	}
	newRules := make([]*Rule, len(r.rules)-len(removed_index))
	m := make(map[int]bool, len(r.rules))
	for _, reomve := range removed_index {
		m[reomve] = true
	}
	start := 0
	for index, rl := range r.rules {
		if !m[index] {
			newRules[start] = rl
			start += 1
		}
	}
	newtargettag2indexmap := make(map[string]int, len(newRules))
	newindex2targettag := make(map[int]string, len(newRules))
	for index, rule := range newRules {
		newtargettag2indexmap[rule.Tag] = index
		newindex2targettag[index] = rule.Tag
	}
	r.rules = newRules
	r.targettag2indexmap = newtargettag2indexmap
	r.index2targettag = newindex2targettag
	runtime.GC()
	return
}

// Init initializes the Router.
func (r *Router) Init(config *Config, d dns.Client, ohm outbound.Manager) error {
	r.domainStrategy = config.DomainStrategy
	r.dns = d

	r.balancers = make(map[string]*Balancer, len(config.BalancingRule))
	r.targettag2indexmap = map[string]int{}
	r.index2targettag = map[int]string{}
	for _, rule := range config.BalancingRule {
		balancer, err := rule.Build(ohm)
		if err != nil {
			return err
		}
		r.balancers[rule.Tag] = balancer
	}

	r.rules = make([]*Rule, 0, len(config.Rule))
	for _, rule := range config.Rule {
		cond, err := rule.BuildCondition()
		if err != nil {
			return err
		}
		rr := &Rule{
			Condition: cond,
			Tag:       rule.GetTag(),
		}
		btag := rule.GetBalancingTag()
		if len(btag) > 0 {
			brule, found := r.balancers[btag]
			if !found {
				return newError("balancer ", btag, " not found")
			}
			rr.Balancer = brule
		}
		r.rules = append(r.rules, rr)
	}

	return nil
}

func (r *Router) PickRoute(ctx context.Context) (string, error) {
	rule, err := r.pickRouteInternal(ctx)
	if err != nil {
		return "", err
	}
	return rule.GetTag()
}

func isDomainOutbound(outbound *session.Outbound) bool {
	return outbound != nil && outbound.Target.IsValid() && outbound.Target.Address.Family().IsDomain()
}

func (r *Router) resolveIP(outbound *session.Outbound) error {
	domain := outbound.Target.Address.Domain()
	ips, err := r.dns.LookupIP(domain)
	if err != nil {
		return err
	}

	outbound.ResolvedIPs = ips
	return nil
}

// PickRoute implements routing.Router.
func (r *Router) pickRouteInternal(ctx context.Context) (*Rule, error) {
	outbound := session.OutboundFromContext(ctx)
	if r.domainStrategy == Config_IpOnDemand && isDomainOutbound(outbound) {
		if err := r.resolveIP(outbound); err != nil {
			newError("failed to resolve IP for domain").Base(err).WriteToLog(session.ExportIDToError(ctx))
		}
	}

	for _, rule := range r.rules {
		if rule.Apply(ctx) {
			return rule, nil
		}
	}

	if r.domainStrategy != Config_IpIfNonMatch || !isDomainOutbound(outbound) {
		return nil, common.ErrNoClue
	}

	if err := r.resolveIP(outbound); err != nil {
		newError("failed to resolve IP for domain").Base(err).WriteToLog(session.ExportIDToError(ctx))
		return nil, common.ErrNoClue
	}

	// Try applying rules again if we have IPs.
	for _, rule := range r.rules {
		if rule.Apply(ctx) {
			return rule, nil
		}
	}

	return nil, common.ErrNoClue
}

// Start implements common.Runnable.
func (*Router) Start() error {
	return nil
}

// Close implements common.Closable.
func (*Router) Close() error {
	return nil
}

// Type implement common.HasType.
func (*Router) Type() interface{} {
	return routing.RouterType()
}
