import{f as n,j as i,c as a,t as r}from"./index-DeD7kkD6.js";import{S as m}from"./shield-check-CVZmQ19h.js";/**
 * @license lucide-react v0.462.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const y=n("UserRound",[["circle",{cx:"12",cy:"8",r:"5",key:"1hypcn"}],["path",{d:"M20 21a8 8 0 0 0-16 0",key:"rfgkzh"}]]);/**
 * @license lucide-react v0.462.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const x=n("Users",[["path",{d:"M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2",key:"1yyitq"}],["circle",{cx:"9",cy:"7",r:"4",key:"nufk8"}],["path",{d:"M22 21v-2a4 4 0 0 0-3-3.87",key:"kshegd"}],["path",{d:"M16 3.13a4 4 0 0 1 0 7.75",key:"1da9ce"}]]);function l(e){return e==="private"||e==="system"?e:"shared"}function p(e){const s=l(e);return s==="private"?r("visibilityPrivate"):s==="system"?r("visibilitySystem"):r("visibilityShared")}function f({resource:e,className:s,showOwner:c=!1}){const t=l(e.visibility),d=t==="private"?y:t==="system"?m:x;return i.jsxs("div",{className:a("flex flex-wrap items-center gap-1.5",s),children:[i.jsxs("span",{className:a("inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium",t==="private"&&"bg-sky-500/12 text-sky-700 dark:text-sky-300",t==="shared"&&"bg-emerald-500/12 text-emerald-700 dark:text-emerald-300",t==="system"&&"bg-amber-500/14 text-amber-800 dark:text-amber-300"),children:[i.jsx(d,{className:"h-3.5 w-3.5"}),p(t)]}),c&&e.owner_user_id?i.jsx("span",{className:"inline-flex max-w-full items-center rounded-full bg-muted px-2.5 py-1 text-[11px] text-muted-foreground",children:i.jsx("span",{className:"truncate",children:e.owner_user_id})}):null]})}export{f as O,l as n};
