import type { MDXComponents } from 'mdx/types';
import defaultMdxComponents from 'fumadocs-ui/mdx';
import { Accordion, Accordions } from 'fumadocs-ui/components/accordion';
import { File, Files, Folder } from 'fumadocs-ui/components/files';
import { Step, Steps } from 'fumadocs-ui/components/steps';
import { Tab, Tabs } from 'fumadocs-ui/components/tabs';
import { TypeTable } from 'fumadocs-ui/components/type-table';
import { StatusBadge, FeatureStatus, ComingSoon, DesignNote, ShortProductTruth, SpecNote, Status, IllustrativeNote } from './status';
import { SourceReference, VerifiedSnippet } from './source';
import { DocsFigure } from './figure';
import { AvailabilityMatrix } from './availability';
import { DocsHomeHero, PathCards } from './home';
import {
  SystemBoundaryExplorer, ChangeLifecycleTimeline, AutonomyPolicyExplorer, HarnessAdapterExplorer, ContextResolverFlow,
  PluginInvocationSequence, EvidenceChainInspector, DeploymentBoundary, VerificationAndRepair, CapabilityMatrix,
} from './diagrams';

export {
  StatusBadge, FeatureStatus, ComingSoon, DesignNote, ShortProductTruth, SpecNote, Status, IllustrativeNote,
  SourceReference, VerifiedSnippet, DocsFigure, AvailabilityMatrix,
};

/** Single registry of every component usable from MDX. Add new components here (and only here). */
export function getMDXComponents(extra?: MDXComponents): MDXComponents {
  return {
    ...defaultMdxComponents,
    Accordion, Accordions, File, Files, Folder, Step, Steps, Tab, Tabs, TypeTable,
    StatusBadge, FeatureStatus, ComingSoon, DesignNote, ShortProductTruth, SpecNote, Status, IllustrativeNote,
    SourceReference, VerifiedSnippet, DocsFigure, AvailabilityMatrix, DocsHomeHero, PathCards,
    SystemBoundaryExplorer, ChangeLifecycleTimeline, AutonomyPolicyExplorer, HarnessAdapterExplorer, ContextResolverFlow,
    PluginInvocationSequence, EvidenceChainInspector, DeploymentBoundary, VerificationAndRepair, CapabilityMatrix,
    ...extra,
  };
}
