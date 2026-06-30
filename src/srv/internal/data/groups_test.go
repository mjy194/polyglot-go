package data

import (
	"context"
	"testing"

	"polyglot/internal/domain"
)

func TestGroupRepositoryUpsertAndAssoc(t *testing.T) {
	store := newTestStore(t)
	repo := store.Groups()
	ctx := context.Background()

	// "default" is seeded by Open.
	def, found, err := repo.GetGroupByName(ctx, "default")
	if err != nil || !found {
		t.Fatalf("default group not seeded: found=%v err=%v", found, err)
	}

	// Upsert a new group.
	g, err := repo.UpsertGroup(ctx, domain.Group{Name: "vip", Ratio: 1.5, Strategy: "round_robin"})
	if err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}
	if g.ID == "" || g.Strategy != "round_robin" {
		t.Fatalf("unexpected group: %+v", g)
	}

	prov, err := store.Providers().UpsertProvider(ctx, domain.Provider{Name: "p", Type: "anthropic", BaseURL: "https://x"})
	if err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	if err := repo.SetGroupProviders(ctx, g.ID, []domain.GroupProvider{
		{ProviderID: prov.ID, Priority: 1},
	}); err != nil {
		t.Fatalf("SetGroupProviders: %v", err)
	}
	links, err := repo.ListGroupProviders(ctx, g.ID)
	if err != nil {
		t.Fatalf("ListGroupProviders: %v", err)
	}
	if len(links) != 1 || links[0].ProviderID != prov.ID {
		t.Fatalf("unexpected links: %+v", links)
	}
	provs, err := repo.ListProvidersForGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("ListProvidersForGroup: %v", err)
	}
	if len(provs) != 1 || provs[0].ID != prov.ID {
		t.Fatalf("expected the provider in the group, got %+v", provs)
	}

	_, _ = repo.ListProvidersForGroup(ctx, def.ID)

	if err := repo.SetProviderGroups(ctx, prov.ID, []domain.GroupProvider{{GroupID: g.ID}}); err != nil {
		t.Fatalf("SetProviderGroups: %v", err)
	}
	pg, _ := repo.ListProviderGroups(ctx, prov.ID)
	if len(pg) != 1 || pg[0].GroupID != g.ID {
		t.Fatalf("expected provider now only in vip, got %+v", pg)
	}

	if err := repo.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	if _, found, _ := repo.GetGroup(ctx, g.ID); found {
		t.Fatalf("group should be deleted")
	}
}
