// Thin re-export shim for backward compatibility.
// CronPage still imports useMarketplaceSkills from this module.
// All marketplace endpoints have been removed; this re-exports from useSkills.
export { useSkills as useMarketplaceSkills } from '@/hooks/useSkills';
export type { SkillItem as MarketplaceSkill } from '@/hooks/useSkills';
