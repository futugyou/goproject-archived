package skillkit

import (
	"context"
	"fmt"

	"github.com/futugyou/openclaw/util"
)

type SkillPackageService struct {
	renderer         SkillTemplateRenderer
	reader           SkillPackageReader
	writer           SkillPackageWriter
	validator        SkillValidator
	critiqueProvider ISkillCritiqueProvider
	runPlanner       SkillRunPlanner
	traceUpdater     SkillTraceUpdater
}

func CreateDefaultSkillPackageService() *SkillPackageService {
	renderer := SkillTemplateRenderer{}
	reader := SkillPackageReader{}
	return &SkillPackageService{
		renderer:         renderer,
		reader:           reader,
		writer:           *NewSkillPackageWriter(renderer),
		validator:        SkillValidator{},
		critiqueProvider: &DeterministicSkillCritiqueProvider{},
		runPlanner:       SkillRunPlanner{},
		traceUpdater:     SkillTraceUpdater{},
	}
}

func (s *SkillPackageService) CreateNew(ctx context.Context, name, category, template, skillsRoot string, force bool) (*SkillPackage, error) {
	var manifest = s.renderer.CreateManifest(name, category, template)
	root, err := s.writer.Create(ctx, *manifest, skillsRoot, force)
	if err != nil {
		return nil, err
	}
	return s.reader.Read(ctx, root, skillsRoot)
}

func (s *SkillPackageService) List(ctx context.Context, skillsRoot string) ([]SkillPackage, error) {
	return s.reader.List(ctx, skillsRoot)
}

func (s *SkillPackageService) Read(ctx context.Context, skillRef, skillsRoot string) (*SkillPackage, error) {
	return s.reader.Read(ctx, skillRef, skillsRoot)
}

func (s *SkillPackageService) Validate(ctx context.Context, skillRef, skillsRoot string) SkillValidationResult {
	return s.validator.Validate(ctx, skillRef, skillsRoot)
}

func (s *SkillPackageService) Generate(ctx context.Context, skillRef, skillsRoot string, force bool) error {
	pkg, err := s.reader.Read(ctx, skillRef, skillsRoot)
	if err != nil {
		return err
	}

	if err := s.writer.GenerateMissing(ctx, *pkg, force); err != nil {
		return nil
	}

	message := "Generated missing skill files."
	if force {
		message = "Regenerated skill files with force."
	}

	return s.traceUpdater.Append(ctx, *pkg, message)
}

func (s *SkillPackageService) Package(ctx context.Context, skillRef, skillsRoot, packagesRoot string, force bool) (string, error) {
	pkg, err := s.reader.Read(ctx, skillRef, skillsRoot)
	if err != nil {
		return "", err
	}

	zipPath, err := s.writer.CreateZip(ctx, *pkg, packagesRoot, force)
	if err != nil {
		return "", err
	}

	if err := s.traceUpdater.Append(ctx, *pkg, fmt.Sprintf("Packaged skill as %s.", zipPath)); err != nil {
		return "", err
	}

	return zipPath, nil
}

func (s *SkillPackageService) PlanRun(ctx context.Context, skillRef, skillsRoot string, inputPaths []string) (*SkillRunPlan, error) {
	pkg, err := s.reader.Read(ctx, skillRef, skillsRoot)
	if err != nil {
		return nil, err
	}

	plan := s.runPlanner.Plan(*pkg, inputPaths)
	return &plan, nil
}

func (s *SkillPackageService) Critique(ctx context.Context, skillRef, skillsRoot string) (string, error) {
	pkg, err := s.reader.Read(ctx, skillRef, skillsRoot)
	if err != nil {
		return "", err
	}

	critique, err := s.critiqueProvider.Critique(ctx, *pkg)
	if err != nil {
		return "", err
	}

	path, err := ResolvePackageFilePath(pkg.RootPath, "critique.md")
	if err != nil {
		return "", err
	}

	if err := util.SaveFile(ctx, path, critique.Markdown); err != nil {
		return "", err
	}

	if err := s.traceUpdater.Append(ctx, *pkg, "Generated deterministic critique."); err != nil {
		return "", err
	}

	return path, nil
}
