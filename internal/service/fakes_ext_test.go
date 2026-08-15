package service

// 本文件为内存 fake（定义在 alert_test.go、deliver_test.go 和 auth_test.go
// 中）补充管理类 API 用到的 store 方法，使 M3 的 service 可以在无数据库的
// 情况下做单元测试

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
)

// --- fakeSourceStore ---

func (f *fakeSourceStore) FindByID(_ context.Context, id int64) (*model.Source, error) {
	for _, s := range f.byToken {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, nil
}

func (f *fakeSourceStore) FindByName(_ context.Context, name string) (*model.Source, error) {
	for _, s := range f.byToken {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, nil
}

func (f *fakeSourceStore) List(_ context.Context, offset, limit int) ([]model.Source, int64, error) {
	all := make([]*model.Source, 0, len(f.byToken))
	for _, s := range f.byToken {
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Source, 0, end-offset)
	for _, s := range all[offset:end] {
		out = append(out, *s)
	}
	return out, total, nil
}

func (f *fakeSourceStore) Create(_ context.Context, s *model.Source) error {
	if s.ID == 0 {
		s.ID = int64(len(f.byToken) + 1)
	}
	f.byToken[s.Token] = s
	return nil
}

func (f *fakeSourceStore) Save(_ context.Context, s *model.Source) error {
	for tok, cur := range f.byToken {
		if cur.ID == s.ID && tok != s.Token {
			delete(f.byToken, tok)
		}
	}
	f.byToken[s.Token] = s
	return nil
}

func (f *fakeSourceStore) Delete(_ context.Context, id int64) error {
	for tok, s := range f.byToken {
		if s.ID == id {
			delete(f.byToken, tok)
		}
	}
	return nil
}

// --- fakeAlertStore ---

func (f *fakeAlertStore) List(_ context.Context, filter AlertFilter) ([]model.Alert, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*model.Alert
	for _, a := range f.byID {
		if filter.Status != "" && a.Status != filter.Status {
			continue
		}
		if filter.Severity != "" && a.Severity != filter.Severity {
			continue
		}
		if filter.Alertname != "" && a.Alertname != filter.Alertname {
			continue
		}
		if filter.Start != nil && a.ReceivedAt.Before(*filter.Start) {
			continue
		}
		if filter.End != nil && a.ReceivedAt.After(*filter.End) {
			continue
		}
		all = append(all, a)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ReceivedAt.After(all[j].ReceivedAt) })
	total := int64(len(all))
	if filter.Offset >= len(all) {
		return nil, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Alert, 0, end-filter.Offset)
	for _, a := range all[filter.Offset:end] {
		out = append(out, *a)
	}
	return out, total, nil
}

func (f *fakeAlertStore) CountBySource(_ context.Context, sourceID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, a := range f.byID {
		if a.SourceID == sourceID {
			n++
		}
	}
	return n, nil
}

func (f *fakeAlertStore) LatestRawPayload(_ context.Context) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *model.Alert
	for _, a := range f.byID {
		if latest == nil || a.ReceivedAt.After(latest.ReceivedAt) {
			latest = a
		}
	}
	if latest == nil {
		return nil, nil
	}
	return latest.RawPayload, nil
}

// --- fakeRuleStore ---

func (f *fakeRuleStore) FindByID(_ context.Context, id int64) (*model.RoutingRule, error) {
	for i := range f.rules {
		if f.rules[i].ID == id {
			r := f.rules[i]
			return &r, nil
		}
	}
	return nil, nil
}

func (f *fakeRuleStore) List(_ context.Context, sourceID *int64, offset, limit int) ([]model.RoutingRule, int64, error) {
	var all []model.RoutingRule
	for _, r := range f.rules {
		if sourceID != nil && r.SourceID != *sourceID {
			continue
		}
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Priority < all[j].Priority })
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (f *fakeRuleStore) Create(_ context.Context, r *model.RoutingRule) error {
	var maxID int64
	for _, cur := range f.rules {
		if cur.ID > maxID {
			maxID = cur.ID
		}
	}
	if r.ID == 0 {
		r.ID = maxID + 1
	}
	f.rules = append(f.rules, *r)
	return nil
}

func (f *fakeRuleStore) Save(_ context.Context, r *model.RoutingRule) error {
	for i := range f.rules {
		if f.rules[i].ID == r.ID {
			f.rules[i] = *r
			return nil
		}
	}
	f.rules = append(f.rules, *r)
	return nil
}

func (f *fakeRuleStore) Delete(_ context.Context, id int64) error {
	out := f.rules[:0]
	for _, r := range f.rules {
		if r.ID != id {
			out = append(out, r)
		}
	}
	f.rules = out
	return nil
}

func (f *fakeRuleStore) CountBySource(_ context.Context, sourceID int64) (int64, error) {
	var n int64
	for _, r := range f.rules {
		if r.SourceID == sourceID {
			n++
		}
	}
	return n, nil
}

func (f *fakeRuleStore) CountByChannel(_ context.Context, channelID int64) (int64, error) {
	var n int64
	for _, r := range f.rules {
		if r.ChannelID == channelID {
			n++
		}
	}
	return n, nil
}

// --- fakeChannelStore ---

func (f *fakeChannelStore) FindByName(_ context.Context, name string) (*model.Channel, error) {
	for _, ch := range f.byID {
		if ch.Name == name {
			return ch, nil
		}
	}
	return nil, nil
}

func (f *fakeChannelStore) List(_ context.Context, offset, limit int) ([]model.Channel, int64, error) {
	all := make([]*model.Channel, 0, len(f.byID))
	for _, ch := range f.byID {
		all = append(all, ch)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Channel, 0, end-offset)
	for _, ch := range all[offset:end] {
		out = append(out, *ch)
	}
	return out, total, nil
}

func (f *fakeChannelStore) Create(_ context.Context, ch *model.Channel) error {
	var maxID int64
	for id := range f.byID {
		if id > maxID {
			maxID = id
		}
	}
	if ch.ID == 0 {
		ch.ID = maxID + 1
	}
	f.byID[ch.ID] = ch
	return nil
}

func (f *fakeChannelStore) Save(_ context.Context, ch *model.Channel) error {
	f.byID[ch.ID] = ch
	return nil
}

func (f *fakeChannelStore) Delete(_ context.Context, id int64) error {
	delete(f.byID, id)
	return nil
}

func (f *fakeChannelStore) CountByTemplate(_ context.Context, templateID int64) (int64, error) {
	var n int64
	for _, ch := range f.byID {
		if ch.TemplateID != nil && *ch.TemplateID == templateID {
			n++
		}
	}
	return n, nil
}

// --- fakeTemplateStore ---

func (f *fakeTemplateStore) FindByName(_ context.Context, channelType, name string) (*model.Template, error) {
	for _, t := range f.byID {
		if t.ChannelType == channelType && t.Name == name {
			return t, nil
		}
	}
	if t := f.builtins[channelType+"/"+name]; t != nil {
		return t, nil
	}
	return nil, nil
}

func (f *fakeTemplateStore) List(_ context.Context, channelType string, offset, limit int) ([]model.Template, int64, error) {
	var all []*model.Template
	for _, t := range f.byID {
		if channelType == "" || t.ChannelType == channelType {
			all = append(all, t)
		}
	}
	for _, t := range f.builtins {
		if channelType == "" || t.ChannelType == channelType {
			all = append(all, t)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Template, 0, end-offset)
	for _, t := range all[offset:end] {
		out = append(out, *t)
	}
	return out, total, nil
}

func (f *fakeTemplateStore) Create(_ context.Context, t *model.Template) error {
	var maxID int64
	for id := range f.byID {
		if id > maxID {
			maxID = id
		}
	}
	for _, bt := range f.builtins {
		if bt.ID > maxID {
			maxID = bt.ID
		}
	}
	if t.ID == 0 {
		t.ID = maxID + 1
	}
	f.byID[t.ID] = t
	return nil
}

func (f *fakeTemplateStore) Save(_ context.Context, t *model.Template) error {
	f.byID[t.ID] = t
	return nil
}

func (f *fakeTemplateStore) Delete(_ context.Context, id int64) error {
	delete(f.byID, id)
	return nil
}

// --- fakeDeliveryStore ---

func (f *fakeDeliveryStore) FindByID(_ context.Context, id int64) (*model.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.rows {
		if d.ID == id {
			return d, nil
		}
	}
	return nil, nil
}

func (f *fakeDeliveryStore) List(_ context.Context, filter DeliveryFilter) ([]model.Delivery, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all []*model.Delivery
	for _, d := range f.rows {
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if filter.ChannelID != nil && d.ChannelID != *filter.ChannelID {
			continue
		}
		if filter.Start != nil && d.SentAt.Before(*filter.Start) {
			continue
		}
		if filter.End != nil && d.SentAt.After(*filter.End) {
			continue
		}
		all = append(all, d)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].SentAt.After(all[j].SentAt) })
	total := int64(len(all))
	if filter.Offset >= len(all) {
		return nil, total, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.Delivery, 0, end-filter.Offset)
	for _, d := range all[filter.Offset:end] {
		out = append(out, *d)
	}
	return out, total, nil
}

func (f *fakeDeliveryStore) ListByAlertID(_ context.Context, alertID int64) ([]model.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Delivery
	for _, d := range f.rows {
		if d.AlertID == alertID {
			out = append(out, *d)
		}
	}
	return out, nil
}

// --- fakeUserStore / fakeTokenStore（定义在 auth_test.go） ---

func (f *fakeUserStore) List(_ context.Context, offset, limit int) ([]model.User, int64, error) {
	all := make([]*model.User, 0, len(f.byID))
	for _, u := range f.byID {
		all = append(all, u)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := int64(len(all))
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	out := make([]model.User, 0, end-offset)
	for _, u := range all[offset:end] {
		out = append(out, *u)
	}
	return out, total, nil
}

func (f *fakeUserStore) UpdateRoleAndStatus(_ context.Context, id int64, role string, enabled bool) error {
	u := f.byID[id]
	u.Role = role
	u.Enabled = enabled
	return nil
}

func (f *fakeUserStore) UpdateProfile(_ context.Context, id int64, name, email, avatarURL string) error {
	u := f.byID[id]
	u.Name = name
	u.Email = email
	u.AvatarURL = avatarURL
	return nil
}

func (f *fakeUserStore) CountEnabledAdmins(_ context.Context) (int64, error) {
	var n int64
	for _, u := range f.byID {
		if u.Role == model.RoleAdmin && u.Enabled {
			n++
		}
	}
	return n, nil
}

func (f *fakeUserStore) Delete(_ context.Context, id int64) error {
	u := f.byID[id]
	if u != nil {
		delete(f.byName, u.Username)
		delete(f.byID, id)
	}
	return nil
}

func (f *fakeTokenStore) DeleteAllForUser(_ context.Context, userID int64) error {
	for hash, t := range f.byHash {
		if t.UserID == userID {
			delete(f.byHash, hash)
		}
	}
	return nil
}

// fakeOAuthIdentityStore 记录 DeleteAllForUser 的调用
type fakeOAuthIdentityStore struct {
	deleted []int64
	byKey   map[string]*model.OAuthIdentity // provider/providerUserID
	nextID  int64
}

func newFakeOAuthIdentityStore() *fakeOAuthIdentityStore {
	return &fakeOAuthIdentityStore{byKey: map[string]*model.OAuthIdentity{}}
}

func (f *fakeOAuthIdentityStore) FindByProviderUserID(_ context.Context, provider, providerUserID string) (*model.OAuthIdentity, error) {
	return f.byKey[provider+"/"+providerUserID], nil
}

func (f *fakeOAuthIdentityStore) Create(_ context.Context, o *model.OAuthIdentity) error {
	f.nextID++
	o.ID = f.nextID
	f.byKey[o.Provider+"/"+o.ProviderUserID] = o
	return nil
}

func (f *fakeOAuthIdentityStore) DeleteAllForUser(_ context.Context, userID int64) error {
	f.deleted = append(f.deleted, userID)
	for k, o := range f.byKey {
		if o.UserID == userID {
			delete(f.byKey, k)
		}
	}
	return nil
}
