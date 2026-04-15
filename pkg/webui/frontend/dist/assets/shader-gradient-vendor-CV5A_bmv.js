import{W as pn,S as hn,P as _n,C as yn,a as xn,A as Cn,M as Pn,V as Tn,b as ce,c as zn,E as wn,R as En,d as bn,e as Be,I as Sn,f as An,D as On,g as Dn,T as Nn,h as Rn}from"./three-vendor-wL_6bxrw.js";import{g as Ln}from"./xterm-vendor-Are6k0PX.js";function Un(o,e){for(var n=0;n<e.length;n++){const t=e[n];if(typeof t!="string"&&!Array.isArray(t)){for(const a in t)if(a!=="default"&&!(a in o)){const s=Object.getOwnPropertyDescriptor(t,a);s&&Object.defineProperty(o,a,s.get?s:{enumerable:!0,get:()=>t[a]})}}}return Object.freeze(Object.defineProperty(o,Symbol.toStringTag,{value:"Module"}))}var De={exports:{}},ve={},Ne={exports:{}},S={};/**
 * @license React
 * react.production.min.js
 *
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */var Ze;function In(){if(Ze)return S;Ze=1;var o=Symbol.for("react.element"),e=Symbol.for("react.portal"),n=Symbol.for("react.fragment"),t=Symbol.for("react.strict_mode"),a=Symbol.for("react.profiler"),s=Symbol.for("react.provider"),l=Symbol.for("react.context"),f=Symbol.for("react.forward_ref"),m=Symbol.for("react.suspense"),p=Symbol.for("react.memo"),P=Symbol.for("react.lazy"),h=Symbol.iterator;function w(r){return r===null||typeof r!="object"?null:(r=h&&r[h]||r["@@iterator"],typeof r=="function"?r:null)}var _={isMounted:function(){return!1},enqueueForceUpdate:function(){},enqueueReplaceState:function(){},enqueueSetState:function(){}},L=Object.assign,E={};function R(r,d,D){this.props=r,this.context=d,this.refs=E,this.updater=D||_}R.prototype.isReactComponent={},R.prototype.setState=function(r,d){if(typeof r!="object"&&typeof r!="function"&&r!=null)throw Error("setState(...): takes an object of state variables to update or a function which returns an object of state variables.");this.updater.enqueueSetState(this,r,d,"setState")},R.prototype.forceUpdate=function(r){this.updater.enqueueForceUpdate(this,r,"forceUpdate")};function c(){}c.prototype=R.prototype;function g(r,d,D){this.props=r,this.context=d,this.refs=E,this.updater=D||_}var v=g.prototype=new c;v.constructor=g,L(v,R.prototype),v.isPureReactComponent=!0;var y=Array.isArray,b=Object.prototype.hasOwnProperty,T={current:null},A={key:!0,ref:!0,__self:!0,__source:!0};function B(r,d,D){var F,U={},k=null,V=null;if(d!=null)for(F in d.ref!==void 0&&(V=d.ref),d.key!==void 0&&(k=""+d.key),d)b.call(d,F)&&!A.hasOwnProperty(F)&&(U[F]=d[F]);var Y=arguments.length-2;if(Y===1)U.children=D;else if(1<Y){for(var H=Array(Y),K=0;K<Y;K++)H[K]=arguments[K+2];U.children=H}if(r&&r.defaultProps)for(F in Y=r.defaultProps,Y)U[F]===void 0&&(U[F]=Y[F]);return{$$typeof:o,type:r,key:k,ref:V,props:U,_owner:T.current}}function te(r,d){return{$$typeof:o,type:r.type,key:d,ref:r.ref,props:r.props,_owner:r._owner}}function ie(r){return typeof r=="object"&&r!==null&&r.$$typeof===o}function Oe(r){var d={"=":"=0",":":"=2"};return"$"+r.replace(/[=:]/g,function(D){return d[D]})}var Te=/\/+/g;function ge(r,d){return typeof r=="object"&&r!==null&&r.key!=null?Oe(""+r.key):d.toString(36)}function le(r,d,D,F,U){var k=typeof r;(k==="undefined"||k==="boolean")&&(r=null);var V=!1;if(r===null)V=!0;else switch(k){case"string":case"number":V=!0;break;case"object":switch(r.$$typeof){case o:case e:V=!0}}if(V)return V=r,U=U(V),r=F===""?"."+ge(V,0):F,y(U)?(D="",r!=null&&(D=r.replace(Te,"$&/")+"/"),le(U,d,D,"",function(K){return K})):U!=null&&(ie(U)&&(U=te(U,D+(!U.key||V&&V.key===U.key?"":(""+U.key).replace(Te,"$&/")+"/")+r)),d.push(U)),1;if(V=0,F=F===""?".":F+":",y(r))for(var Y=0;Y<r.length;Y++){k=r[Y];var H=F+ge(k,Y);V+=le(k,d,D,H,U)}else if(H=w(r),typeof H=="function")for(r=H.call(r),Y=0;!(k=r.next()).done;)k=k.value,H=F+ge(k,Y++),V+=le(k,d,D,H,U);else if(k==="object")throw d=String(r),Error("Objects are not valid as a React child (found: "+(d==="[object Object]"?"object with keys {"+Object.keys(r).join(", ")+"}":d)+"). If you meant to render a collection of children, use an array instead.");return V}function re(r,d,D){if(r==null)return r;var F=[],U=0;return le(r,F,"","",function(k){return d.call(D,k,U++)}),F}function gn(r){if(r._status===-1){var d=r._result;d=d(),d.then(function(D){(r._status===0||r._status===-1)&&(r._status=1,r._result=D)},function(D){(r._status===0||r._status===-1)&&(r._status=2,r._result=D)}),r._status===-1&&(r._status=0,r._result=d)}if(r._status===1)return r._result.default;throw r._result}var q={current:null},ze={transition:null},vn={ReactCurrentDispatcher:q,ReactCurrentBatchConfig:ze,ReactCurrentOwner:T};function Ve(){throw Error("act(...) is not supported in production builds of React.")}return S.Children={map:re,forEach:function(r,d,D){re(r,function(){d.apply(this,arguments)},D)},count:function(r){var d=0;return re(r,function(){d++}),d},toArray:function(r){return re(r,function(d){return d})||[]},only:function(r){if(!ie(r))throw Error("React.Children.only expected to receive a single React element child.");return r}},S.Component=R,S.Fragment=n,S.Profiler=a,S.PureComponent=g,S.StrictMode=t,S.Suspense=m,S.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED=vn,S.act=Ve,S.cloneElement=function(r,d,D){if(r==null)throw Error("React.cloneElement(...): The argument must be a React element, but you passed "+r+".");var F=L({},r.props),U=r.key,k=r.ref,V=r._owner;if(d!=null){if(d.ref!==void 0&&(k=d.ref,V=T.current),d.key!==void 0&&(U=""+d.key),r.type&&r.type.defaultProps)var Y=r.type.defaultProps;for(H in d)b.call(d,H)&&!A.hasOwnProperty(H)&&(F[H]=d[H]===void 0&&Y!==void 0?Y[H]:d[H])}var H=arguments.length-2;if(H===1)F.children=D;else if(1<H){Y=Array(H);for(var K=0;K<H;K++)Y[K]=arguments[K+2];F.children=Y}return{$$typeof:o,type:r.type,key:U,ref:k,props:F,_owner:V}},S.createContext=function(r){return r={$$typeof:l,_currentValue:r,_currentValue2:r,_threadCount:0,Provider:null,Consumer:null,_defaultValue:null,_globalName:null},r.Provider={$$typeof:s,_context:r},r.Consumer=r},S.createElement=B,S.createFactory=function(r){var d=B.bind(null,r);return d.type=r,d},S.createRef=function(){return{current:null}},S.forwardRef=function(r){return{$$typeof:f,render:r}},S.isValidElement=ie,S.lazy=function(r){return{$$typeof:P,_payload:{_status:-1,_result:r},_init:gn}},S.memo=function(r,d){return{$$typeof:p,type:r,compare:d===void 0?null:d}},S.startTransition=function(r){var d=ze.transition;ze.transition={};try{r()}finally{ze.transition=d}},S.unstable_act=Ve,S.useCallback=function(r,d){return q.current.useCallback(r,d)},S.useContext=function(r){return q.current.useContext(r)},S.useDebugValue=function(){},S.useDeferredValue=function(r){return q.current.useDeferredValue(r)},S.useEffect=function(r,d){return q.current.useEffect(r,d)},S.useId=function(){return q.current.useId()},S.useImperativeHandle=function(r,d,D){return q.current.useImperativeHandle(r,d,D)},S.useInsertionEffect=function(r,d){return q.current.useInsertionEffect(r,d)},S.useLayoutEffect=function(r,d){return q.current.useLayoutEffect(r,d)},S.useMemo=function(r,d){return q.current.useMemo(r,d)},S.useReducer=function(r,d,D){return q.current.useReducer(r,d,D)},S.useRef=function(r){return q.current.useRef(r)},S.useState=function(r){return q.current.useState(r)},S.useSyncExternalStore=function(r,d,D){return q.current.useSyncExternalStore(r,d,D)},S.useTransition=function(){return q.current.useTransition()},S.version="18.3.1",S}var qe;function fn(){return qe||(qe=1,Ne.exports=In()),Ne.exports}/**
 * @license React
 * react-jsx-runtime.production.min.js
 *
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */var Ge;function Fn(){if(Ge)return ve;Ge=1;var o=fn(),e=Symbol.for("react.element"),n=Symbol.for("react.fragment"),t=Object.prototype.hasOwnProperty,a=o.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED.ReactCurrentOwner,s={key:!0,ref:!0,__self:!0,__source:!0};function l(f,m,p){var P,h={},w=null,_=null;p!==void 0&&(w=""+p),m.key!==void 0&&(w=""+m.key),m.ref!==void 0&&(_=m.ref);for(P in m)t.call(m,P)&&!s.hasOwnProperty(P)&&(h[P]=m[P]);if(f&&f.defaultProps)for(P in m=f.defaultProps,m)h[P]===void 0&&(h[P]=m[P]);return{$$typeof:e,type:f,key:w,ref:_,props:h,_owner:a.current}}return ve.Fragment=n,ve.jsx=l,ve.jsxs=l,ve}var We;function Mn(){return We||(We=1,De.exports=Fn()),De.exports}var Xe=Mn(),j=fn();const Hn=Ln(j),Kt=Un({__proto__:null,default:Hn},[j]);/*!
 * camera-controls
 * https://github.com/yomotsu/camera-controls
 * (c) 2017 @yomotsu
 * Released under the MIT License.
 */const M={LEFT:1,RIGHT:2,MIDDLE:4},i=Object.freeze({NONE:0,ROTATE:1,TRUCK:2,SCREEN_PAN:4,OFFSET:8,DOLLY:16,ZOOM:32,TOUCH_ROTATE:64,TOUCH_TRUCK:128,TOUCH_SCREEN_PAN:256,TOUCH_OFFSET:512,TOUCH_DOLLY:1024,TOUCH_ZOOM:2048,TOUCH_DOLLY_TRUCK:4096,TOUCH_DOLLY_SCREEN_PAN:8192,TOUCH_DOLLY_OFFSET:16384,TOUCH_DOLLY_ROTATE:32768,TOUCH_ZOOM_TRUCK:65536,TOUCH_ZOOM_OFFSET:131072,TOUCH_ZOOM_SCREEN_PAN:262144,TOUCH_ZOOM_ROTATE:524288}),fe={NONE:0,IN:1,OUT:-1};function ae(o){return o.isPerspectiveCamera}function oe(o){return o.isOrthographicCamera}const ue=Math.PI*2,je=Math.PI/2,un=1e-5,pe=Math.PI/180;function Q(o,e,n){return Math.max(e,Math.min(n,o))}function I(o,e=un){return Math.abs(o)<e}function N(o,e,n=un){return I(o-e,n)}function $e(o,e){return Math.round(o/e)*e}function he(o){return isFinite(o)?o:o<0?-Number.MAX_VALUE:Number.MAX_VALUE}function _e(o){return Math.abs(o)<Number.MAX_VALUE?o:o*(1/0)}function we(o,e,n,t,a=1/0,s){t=Math.max(1e-4,t);const l=2/t,f=l*s,m=1/(1+f+.48*f*f+.235*f*f*f);let p=o-e;const P=e,h=a*t;p=Q(p,-h,h),e=o-p;const w=(n.value+l*p)*s;n.value=(n.value-l*w)*m;let _=e+(p+w)*m;return P-o>0==_>P&&(_=P,n.value=(_-P)/s),_}function Ke(o,e,n,t,a=1/0,s,l){t=Math.max(1e-4,t);const f=2/t,m=f*s,p=1/(1+m+.48*m*m+.235*m*m*m);let P=e.x,h=e.y,w=e.z,_=o.x-P,L=o.y-h,E=o.z-w;const R=P,c=h,g=w,v=a*t,y=v*v,b=_*_+L*L+E*E;if(b>y){const re=Math.sqrt(b);_=_/re*v,L=L/re*v,E=E/re*v}P=o.x-_,h=o.y-L,w=o.z-E;const T=(n.x+f*_)*s,A=(n.y+f*L)*s,B=(n.z+f*E)*s;n.x=(n.x-f*T)*p,n.y=(n.y-f*A)*p,n.z=(n.z-f*B)*p,l.x=P+(_+T)*p,l.y=h+(L+A)*p,l.z=w+(E+B)*p;const te=R-o.x,ie=c-o.y,Oe=g-o.z,Te=l.x-R,ge=l.y-c,le=l.z-g;return te*Te+ie*ge+Oe*le>0&&(l.x=R,l.y=c,l.z=g,n.x=(l.x-R)/s,n.y=(l.y-c)/s,n.z=(l.z-g)/s),l}function Re(o,e){e.set(0,0),o.forEach(n=>{e.x+=n.clientX,e.y+=n.clientY}),e.x/=o.length,e.y/=o.length}function Le(o,e){return oe(o)?(console.warn(`${e} is not supported in OrthographicCamera`),!0):!1}class kn{constructor(){this._listeners={}}addEventListener(e,n){const t=this._listeners;t[e]===void 0&&(t[e]=[]),t[e].indexOf(n)===-1&&t[e].push(n)}hasEventListener(e,n){const t=this._listeners;return t[e]!==void 0&&t[e].indexOf(n)!==-1}removeEventListener(e,n){const a=this._listeners[e];if(a!==void 0){const s=a.indexOf(n);s!==-1&&a.splice(s,1)}}removeAllEventListeners(e){if(!e){this._listeners={};return}Array.isArray(this._listeners[e])&&(this._listeners[e].length=0)}dispatchEvent(e){const t=this._listeners[e.type];if(t!==void 0){e.target=this;const a=t.slice(0);for(let s=0,l=a.length;s<l;s++)a[s].call(this,e)}}}var Ue;const Yn="2.10.1",Ee=1/8,Vn=/Mac/.test((Ue=globalThis==null?void 0:globalThis.navigator)===null||Ue===void 0?void 0:Ue.platform);let C,Qe,be,Ie,G,z,O,de,ye,J,ee,se,Je,en,$,xe,me,nn,Fe,tn,Me,He,Se;class W extends kn{static install(e){C=e.THREE,Qe=Object.freeze(new C.Vector3(0,0,0)),be=Object.freeze(new C.Vector3(0,1,0)),Ie=Object.freeze(new C.Vector3(0,0,1)),G=new C.Vector2,z=new C.Vector3,O=new C.Vector3,de=new C.Vector3,ye=new C.Vector3,J=new C.Vector3,ee=new C.Vector3,se=new C.Vector3,Je=new C.Vector3,en=new C.Vector3,$=new C.Spherical,xe=new C.Spherical,me=new C.Box3,nn=new C.Box3,Fe=new C.Sphere,tn=new C.Quaternion,Me=new C.Quaternion,He=new C.Matrix4,Se=new C.Raycaster}static get ACTION(){return i}set verticalDragToForward(e){console.warn("camera-controls: `verticalDragToForward` was removed. Use `mouseButtons.left = CameraControls.ACTION.SCREEN_PAN` instead.")}constructor(e,n){super(),this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.minDistance=Number.EPSILON,this.maxDistance=1/0,this.infinityDolly=!1,this.minZoom=.01,this.maxZoom=1/0,this.smoothTime=.25,this.draggingSmoothTime=.125,this.maxSpeed=1/0,this.azimuthRotateSpeed=1,this.polarRotateSpeed=1,this.dollySpeed=1,this.dollyDragInverted=!1,this.truckSpeed=2,this.dollyToCursor=!1,this.dragToOffset=!1,this.boundaryFriction=0,this.restThreshold=.01,this.colliderMeshes=[],this.cancel=()=>{},this._enabled=!0,this._state=i.NONE,this._viewport=null,this._changedDolly=0,this._changedZoom=0,this._hasRested=!0,this._boundaryEnclosesCamera=!1,this._needsUpdate=!0,this._updatedLastTime=!1,this._elementRect=new DOMRect,this._isDragging=!1,this._dragNeedsUpdate=!0,this._activePointers=[],this._lockedPointer=null,this._interactiveArea=new DOMRect(0,0,1,1),this._isUserControllingRotate=!1,this._isUserControllingDolly=!1,this._isUserControllingTruck=!1,this._isUserControllingOffset=!1,this._isUserControllingZoom=!1,this._lastDollyDirection=fe.NONE,this._thetaVelocity={value:0},this._phiVelocity={value:0},this._radiusVelocity={value:0},this._targetVelocity=new C.Vector3,this._focalOffsetVelocity=new C.Vector3,this._zoomVelocity={value:0},this._truckInternal=(c,g,v,y)=>{let b,T;if(ae(this._camera)){const A=z.copy(this._camera.position).sub(this._target),B=this._camera.getEffectiveFOV()*pe,te=A.length()*Math.tan(B*.5);b=this.truckSpeed*c*te/this._elementRect.height,T=this.truckSpeed*g*te/this._elementRect.height}else if(oe(this._camera)){const A=this._camera;b=this.truckSpeed*c*(A.right-A.left)/A.zoom/this._elementRect.width,T=this.truckSpeed*g*(A.top-A.bottom)/A.zoom/this._elementRect.height}else return;y?(v?this.setFocalOffset(this._focalOffsetEnd.x+b,this._focalOffsetEnd.y,this._focalOffsetEnd.z,!0):this.truck(b,0,!0),this.forward(-T,!0)):v?this.setFocalOffset(this._focalOffsetEnd.x+b,this._focalOffsetEnd.y+T,this._focalOffsetEnd.z,!0):this.truck(b,T,!0)},this._rotateInternal=(c,g)=>{const v=ue*this.azimuthRotateSpeed*c/this._elementRect.height,y=ue*this.polarRotateSpeed*g/this._elementRect.height;this.rotate(v,y,!0)},this._dollyInternal=(c,g,v)=>{const y=Math.pow(.95,-c*this.dollySpeed),b=this._sphericalEnd.radius,T=this._sphericalEnd.radius*y,A=Q(T,this.minDistance,this.maxDistance),B=A-T;this.infinityDolly&&this.dollyToCursor?this._dollyToNoClamp(T,!0):this.infinityDolly&&!this.dollyToCursor?(this.dollyInFixed(B,!0),this._dollyToNoClamp(A,!0)):this._dollyToNoClamp(A,!0),this.dollyToCursor&&(this._changedDolly+=(this.infinityDolly?T:A)-b,this._dollyControlCoord.set(g,v)),this._lastDollyDirection=Math.sign(-c)},this._zoomInternal=(c,g,v)=>{const y=Math.pow(.95,c*this.dollySpeed),b=this._zoom,T=this._zoom*y;this.zoomTo(T,!0),this.dollyToCursor&&(this._changedZoom+=T-b,this._dollyControlCoord.set(g,v))},typeof C>"u"&&console.error("camera-controls: `THREE` is undefined. You must first run `CameraControls.install( { THREE: THREE } )`. Check the docs for further information."),this._camera=e,this._yAxisUpSpace=new C.Quaternion().setFromUnitVectors(this._camera.up,be),this._yAxisUpSpaceInverse=this._yAxisUpSpace.clone().invert(),this._state=i.NONE,this._target=new C.Vector3,this._targetEnd=this._target.clone(),this._focalOffset=new C.Vector3,this._focalOffsetEnd=this._focalOffset.clone(),this._spherical=new C.Spherical().setFromVector3(z.copy(this._camera.position).applyQuaternion(this._yAxisUpSpace)),this._sphericalEnd=this._spherical.clone(),this._lastDistance=this._spherical.radius,this._zoom=this._camera.zoom,this._zoomEnd=this._zoom,this._lastZoom=this._zoom,this._nearPlaneCorners=[new C.Vector3,new C.Vector3,new C.Vector3,new C.Vector3],this._updateNearPlaneCorners(),this._boundary=new C.Box3(new C.Vector3(-1/0,-1/0,-1/0),new C.Vector3(1/0,1/0,1/0)),this._cameraUp0=this._camera.up.clone(),this._target0=this._target.clone(),this._position0=this._camera.position.clone(),this._zoom0=this._zoom,this._focalOffset0=this._focalOffset.clone(),this._dollyControlCoord=new C.Vector2,this.mouseButtons={left:i.ROTATE,middle:i.DOLLY,right:i.TRUCK,wheel:ae(this._camera)?i.DOLLY:oe(this._camera)?i.ZOOM:i.NONE},this.touches={one:i.TOUCH_ROTATE,two:ae(this._camera)?i.TOUCH_DOLLY_TRUCK:oe(this._camera)?i.TOUCH_ZOOM_TRUCK:i.NONE,three:i.TOUCH_TRUCK};const t=new C.Vector2,a=new C.Vector2,s=new C.Vector2,l=c=>{if(!this._enabled||!this._domElement)return;if(this._interactiveArea.left!==0||this._interactiveArea.top!==0||this._interactiveArea.width!==1||this._interactiveArea.height!==1){const y=this._domElement.getBoundingClientRect(),b=c.clientX/y.width,T=c.clientY/y.height;if(b<this._interactiveArea.left||b>this._interactiveArea.right||T<this._interactiveArea.top||T>this._interactiveArea.bottom)return}const g=c.pointerType!=="mouse"?null:(c.buttons&M.LEFT)===M.LEFT?M.LEFT:(c.buttons&M.MIDDLE)===M.MIDDLE?M.MIDDLE:(c.buttons&M.RIGHT)===M.RIGHT?M.RIGHT:null;if(g!==null){const y=this._findPointerByMouseButton(g);y&&this._disposePointer(y)}if((c.buttons&M.LEFT)===M.LEFT&&this._lockedPointer)return;const v={pointerId:c.pointerId,clientX:c.clientX,clientY:c.clientY,deltaX:0,deltaY:0,mouseButton:g};this._activePointers.push(v),this._domElement.ownerDocument.removeEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.removeEventListener("pointerup",m),this._domElement.ownerDocument.addEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.addEventListener("pointerup",m),this._isDragging=!0,w(c)},f=c=>{c.cancelable&&c.preventDefault();const g=c.pointerId,v=this._lockedPointer||this._findPointerById(g);if(v){if(v.clientX=c.clientX,v.clientY=c.clientY,v.deltaX=c.movementX,v.deltaY=c.movementY,this._state=0,c.pointerType==="touch")switch(this._activePointers.length){case 1:this._state=this.touches.one;break;case 2:this._state=this.touches.two;break;case 3:this._state=this.touches.three;break}else(!this._isDragging&&this._lockedPointer||this._isDragging&&(c.buttons&M.LEFT)===M.LEFT)&&(this._state=this._state|this.mouseButtons.left),this._isDragging&&(c.buttons&M.MIDDLE)===M.MIDDLE&&(this._state=this._state|this.mouseButtons.middle),this._isDragging&&(c.buttons&M.RIGHT)===M.RIGHT&&(this._state=this._state|this.mouseButtons.right);_()}},m=c=>{const g=this._findPointerById(c.pointerId);if(!(g&&g===this._lockedPointer)){if(g&&this._disposePointer(g),c.pointerType==="touch")switch(this._activePointers.length){case 0:this._state=i.NONE;break;case 1:this._state=this.touches.one;break;case 2:this._state=this.touches.two;break;case 3:this._state=this.touches.three;break}else this._state=i.NONE;L()}};let p=-1;const P=c=>{if(!this._domElement||!this._enabled||this.mouseButtons.wheel===i.NONE)return;if(this._interactiveArea.left!==0||this._interactiveArea.top!==0||this._interactiveArea.width!==1||this._interactiveArea.height!==1){const T=this._domElement.getBoundingClientRect(),A=c.clientX/T.width,B=c.clientY/T.height;if(A<this._interactiveArea.left||A>this._interactiveArea.right||B<this._interactiveArea.top||B>this._interactiveArea.bottom)return}if(c.preventDefault(),this.dollyToCursor||this.mouseButtons.wheel===i.ROTATE||this.mouseButtons.wheel===i.TRUCK){const T=performance.now();p-T<1e3&&this._getClientRect(this._elementRect),p=T}const g=Vn?-1:-3,v=c.deltaMode===1||c.ctrlKey?c.deltaY/g:c.deltaY/(g*10),y=this.dollyToCursor?(c.clientX-this._elementRect.x)/this._elementRect.width*2-1:0,b=this.dollyToCursor?(c.clientY-this._elementRect.y)/this._elementRect.height*-2+1:0;switch(this.mouseButtons.wheel){case i.ROTATE:{this._rotateInternal(c.deltaX,c.deltaY),this._isUserControllingRotate=!0;break}case i.TRUCK:{this._truckInternal(c.deltaX,c.deltaY,!1,!1),this._isUserControllingTruck=!0;break}case i.SCREEN_PAN:{this._truckInternal(c.deltaX,c.deltaY,!1,!0),this._isUserControllingTruck=!0;break}case i.OFFSET:{this._truckInternal(c.deltaX,c.deltaY,!0,!1),this._isUserControllingOffset=!0;break}case i.DOLLY:{this._dollyInternal(-v,y,b),this._isUserControllingDolly=!0;break}case i.ZOOM:{this._zoomInternal(-v,y,b),this._isUserControllingZoom=!0;break}}this.dispatchEvent({type:"control"})},h=c=>{if(!(!this._domElement||!this._enabled)){if(this.mouseButtons.right===W.ACTION.NONE){const g=c instanceof PointerEvent?c.pointerId:0,v=this._findPointerById(g);v&&this._disposePointer(v),this._domElement.ownerDocument.removeEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.removeEventListener("pointerup",m);return}c.preventDefault()}},w=c=>{if(!this._enabled)return;if(Re(this._activePointers,G),this._getClientRect(this._elementRect),t.copy(G),a.copy(G),this._activePointers.length>=2){const v=G.x-this._activePointers[1].clientX,y=G.y-this._activePointers[1].clientY,b=Math.sqrt(v*v+y*y);s.set(0,b);const T=(this._activePointers[0].clientX+this._activePointers[1].clientX)*.5,A=(this._activePointers[0].clientY+this._activePointers[1].clientY)*.5;a.set(T,A)}if(this._state=0,!c)this._lockedPointer&&(this._state=this._state|this.mouseButtons.left);else if("pointerType"in c&&c.pointerType==="touch")switch(this._activePointers.length){case 1:this._state=this.touches.one;break;case 2:this._state=this.touches.two;break;case 3:this._state=this.touches.three;break}else!this._lockedPointer&&(c.buttons&M.LEFT)===M.LEFT&&(this._state=this._state|this.mouseButtons.left),(c.buttons&M.MIDDLE)===M.MIDDLE&&(this._state=this._state|this.mouseButtons.middle),(c.buttons&M.RIGHT)===M.RIGHT&&(this._state=this._state|this.mouseButtons.right);((this._state&i.ROTATE)===i.ROTATE||(this._state&i.TOUCH_ROTATE)===i.TOUCH_ROTATE||(this._state&i.TOUCH_DOLLY_ROTATE)===i.TOUCH_DOLLY_ROTATE||(this._state&i.TOUCH_ZOOM_ROTATE)===i.TOUCH_ZOOM_ROTATE)&&(this._sphericalEnd.theta=this._spherical.theta,this._sphericalEnd.phi=this._spherical.phi,this._thetaVelocity.value=0,this._phiVelocity.value=0),((this._state&i.TRUCK)===i.TRUCK||(this._state&i.SCREEN_PAN)===i.SCREEN_PAN||(this._state&i.TOUCH_TRUCK)===i.TOUCH_TRUCK||(this._state&i.TOUCH_SCREEN_PAN)===i.TOUCH_SCREEN_PAN||(this._state&i.TOUCH_DOLLY_TRUCK)===i.TOUCH_DOLLY_TRUCK||(this._state&i.TOUCH_DOLLY_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN||(this._state&i.TOUCH_ZOOM_TRUCK)===i.TOUCH_ZOOM_TRUCK||(this._state&i.TOUCH_ZOOM_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN)&&(this._targetEnd.copy(this._target),this._targetVelocity.set(0,0,0)),((this._state&i.DOLLY)===i.DOLLY||(this._state&i.TOUCH_DOLLY)===i.TOUCH_DOLLY||(this._state&i.TOUCH_DOLLY_TRUCK)===i.TOUCH_DOLLY_TRUCK||(this._state&i.TOUCH_DOLLY_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN||(this._state&i.TOUCH_DOLLY_OFFSET)===i.TOUCH_DOLLY_OFFSET||(this._state&i.TOUCH_DOLLY_ROTATE)===i.TOUCH_DOLLY_ROTATE)&&(this._sphericalEnd.radius=this._spherical.radius,this._radiusVelocity.value=0),((this._state&i.ZOOM)===i.ZOOM||(this._state&i.TOUCH_ZOOM)===i.TOUCH_ZOOM||(this._state&i.TOUCH_ZOOM_TRUCK)===i.TOUCH_ZOOM_TRUCK||(this._state&i.TOUCH_ZOOM_SCREEN_PAN)===i.TOUCH_ZOOM_SCREEN_PAN||(this._state&i.TOUCH_ZOOM_OFFSET)===i.TOUCH_ZOOM_OFFSET||(this._state&i.TOUCH_ZOOM_ROTATE)===i.TOUCH_ZOOM_ROTATE)&&(this._zoomEnd=this._zoom,this._zoomVelocity.value=0),((this._state&i.OFFSET)===i.OFFSET||(this._state&i.TOUCH_OFFSET)===i.TOUCH_OFFSET||(this._state&i.TOUCH_DOLLY_OFFSET)===i.TOUCH_DOLLY_OFFSET||(this._state&i.TOUCH_ZOOM_OFFSET)===i.TOUCH_ZOOM_OFFSET)&&(this._focalOffsetEnd.copy(this._focalOffset),this._focalOffsetVelocity.set(0,0,0)),this.dispatchEvent({type:"controlstart"})},_=()=>{if(!this._enabled||!this._dragNeedsUpdate)return;this._dragNeedsUpdate=!1,Re(this._activePointers,G);const g=this._domElement&&this._domElement.ownerDocument.pointerLockElement===this._domElement?this._lockedPointer||this._activePointers[0]:null,v=g?-g.deltaX:a.x-G.x,y=g?-g.deltaY:a.y-G.y;if(a.copy(G),((this._state&i.ROTATE)===i.ROTATE||(this._state&i.TOUCH_ROTATE)===i.TOUCH_ROTATE||(this._state&i.TOUCH_DOLLY_ROTATE)===i.TOUCH_DOLLY_ROTATE||(this._state&i.TOUCH_ZOOM_ROTATE)===i.TOUCH_ZOOM_ROTATE)&&(this._rotateInternal(v,y),this._isUserControllingRotate=!0),(this._state&i.DOLLY)===i.DOLLY||(this._state&i.ZOOM)===i.ZOOM){const b=this.dollyToCursor?(t.x-this._elementRect.x)/this._elementRect.width*2-1:0,T=this.dollyToCursor?(t.y-this._elementRect.y)/this._elementRect.height*-2+1:0,A=this.dollyDragInverted?-1:1;(this._state&i.DOLLY)===i.DOLLY?(this._dollyInternal(A*y*Ee,b,T),this._isUserControllingDolly=!0):(this._zoomInternal(A*y*Ee,b,T),this._isUserControllingZoom=!0)}if((this._state&i.TOUCH_DOLLY)===i.TOUCH_DOLLY||(this._state&i.TOUCH_ZOOM)===i.TOUCH_ZOOM||(this._state&i.TOUCH_DOLLY_TRUCK)===i.TOUCH_DOLLY_TRUCK||(this._state&i.TOUCH_ZOOM_TRUCK)===i.TOUCH_ZOOM_TRUCK||(this._state&i.TOUCH_DOLLY_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN||(this._state&i.TOUCH_ZOOM_SCREEN_PAN)===i.TOUCH_ZOOM_SCREEN_PAN||(this._state&i.TOUCH_DOLLY_OFFSET)===i.TOUCH_DOLLY_OFFSET||(this._state&i.TOUCH_ZOOM_OFFSET)===i.TOUCH_ZOOM_OFFSET||(this._state&i.TOUCH_DOLLY_ROTATE)===i.TOUCH_DOLLY_ROTATE||(this._state&i.TOUCH_ZOOM_ROTATE)===i.TOUCH_ZOOM_ROTATE){const b=G.x-this._activePointers[1].clientX,T=G.y-this._activePointers[1].clientY,A=Math.sqrt(b*b+T*T),B=s.y-A;s.set(0,A);const te=this.dollyToCursor?(a.x-this._elementRect.x)/this._elementRect.width*2-1:0,ie=this.dollyToCursor?(a.y-this._elementRect.y)/this._elementRect.height*-2+1:0;(this._state&i.TOUCH_DOLLY)===i.TOUCH_DOLLY||(this._state&i.TOUCH_DOLLY_ROTATE)===i.TOUCH_DOLLY_ROTATE||(this._state&i.TOUCH_DOLLY_TRUCK)===i.TOUCH_DOLLY_TRUCK||(this._state&i.TOUCH_DOLLY_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN||(this._state&i.TOUCH_DOLLY_OFFSET)===i.TOUCH_DOLLY_OFFSET?(this._dollyInternal(B*Ee,te,ie),this._isUserControllingDolly=!0):(this._zoomInternal(B*Ee,te,ie),this._isUserControllingZoom=!0)}((this._state&i.TRUCK)===i.TRUCK||(this._state&i.TOUCH_TRUCK)===i.TOUCH_TRUCK||(this._state&i.TOUCH_DOLLY_TRUCK)===i.TOUCH_DOLLY_TRUCK||(this._state&i.TOUCH_ZOOM_TRUCK)===i.TOUCH_ZOOM_TRUCK)&&(this._truckInternal(v,y,!1,!1),this._isUserControllingTruck=!0),((this._state&i.SCREEN_PAN)===i.SCREEN_PAN||(this._state&i.TOUCH_SCREEN_PAN)===i.TOUCH_SCREEN_PAN||(this._state&i.TOUCH_DOLLY_SCREEN_PAN)===i.TOUCH_DOLLY_SCREEN_PAN||(this._state&i.TOUCH_ZOOM_SCREEN_PAN)===i.TOUCH_ZOOM_SCREEN_PAN)&&(this._truckInternal(v,y,!1,!0),this._isUserControllingTruck=!0),((this._state&i.OFFSET)===i.OFFSET||(this._state&i.TOUCH_OFFSET)===i.TOUCH_OFFSET||(this._state&i.TOUCH_DOLLY_OFFSET)===i.TOUCH_DOLLY_OFFSET||(this._state&i.TOUCH_ZOOM_OFFSET)===i.TOUCH_ZOOM_OFFSET)&&(this._truckInternal(v,y,!0,!1),this._isUserControllingOffset=!0),this.dispatchEvent({type:"control"})},L=()=>{Re(this._activePointers,G),a.copy(G),this._dragNeedsUpdate=!1,(this._activePointers.length===0||this._activePointers.length===1&&this._activePointers[0]===this._lockedPointer)&&(this._isDragging=!1),this._activePointers.length===0&&this._domElement&&(this._domElement.ownerDocument.removeEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.removeEventListener("pointerup",m),this.dispatchEvent({type:"controlend"}))};this.lockPointer=()=>{!this._enabled||!this._domElement||(this.cancel(),this._lockedPointer={pointerId:-1,clientX:0,clientY:0,deltaX:0,deltaY:0,mouseButton:null},this._activePointers.push(this._lockedPointer),this._domElement.ownerDocument.removeEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.removeEventListener("pointerup",m),this._domElement.requestPointerLock(),this._domElement.ownerDocument.addEventListener("pointerlockchange",E),this._domElement.ownerDocument.addEventListener("pointerlockerror",R),this._domElement.ownerDocument.addEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.addEventListener("pointerup",m),w())},this.unlockPointer=()=>{var c,g,v;this._lockedPointer!==null&&(this._disposePointer(this._lockedPointer),this._lockedPointer=null),(c=this._domElement)===null||c===void 0||c.ownerDocument.exitPointerLock(),(g=this._domElement)===null||g===void 0||g.ownerDocument.removeEventListener("pointerlockchange",E),(v=this._domElement)===null||v===void 0||v.ownerDocument.removeEventListener("pointerlockerror",R),this.cancel()};const E=()=>{this._domElement&&this._domElement.ownerDocument.pointerLockElement===this._domElement||this.unlockPointer()},R=()=>{this.unlockPointer()};this._addAllEventListeners=c=>{this._domElement=c,this._domElement.style.touchAction="none",this._domElement.style.userSelect="none",this._domElement.style.webkitUserSelect="none",this._domElement.addEventListener("pointerdown",l),this._domElement.addEventListener("pointercancel",m),this._domElement.addEventListener("wheel",P,{passive:!1}),this._domElement.addEventListener("contextmenu",h)},this._removeAllEventListeners=()=>{this._domElement&&(this._domElement.style.touchAction="",this._domElement.style.userSelect="",this._domElement.style.webkitUserSelect="",this._domElement.removeEventListener("pointerdown",l),this._domElement.removeEventListener("pointercancel",m),this._domElement.removeEventListener("wheel",P,{passive:!1}),this._domElement.removeEventListener("contextmenu",h),this._domElement.ownerDocument.removeEventListener("pointermove",f,{passive:!1}),this._domElement.ownerDocument.removeEventListener("pointerup",m),this._domElement.ownerDocument.removeEventListener("pointerlockchange",E),this._domElement.ownerDocument.removeEventListener("pointerlockerror",R))},this.cancel=()=>{this._state!==i.NONE&&(this._state=i.NONE,this._activePointers.length=0,L())},n&&this.connect(n),this.update(0)}get camera(){return this._camera}set camera(e){this._camera=e,this.updateCameraUp(),this._camera.updateProjectionMatrix(),this._updateNearPlaneCorners(),this._needsUpdate=!0}get enabled(){return this._enabled}set enabled(e){this._enabled=e,this._domElement&&(e?(this._domElement.style.touchAction="none",this._domElement.style.userSelect="none",this._domElement.style.webkitUserSelect="none"):(this.cancel(),this._domElement.style.touchAction="",this._domElement.style.userSelect="",this._domElement.style.webkitUserSelect=""))}get active(){return!this._hasRested}get currentAction(){return this._state}get distance(){return this._spherical.radius}set distance(e){this._spherical.radius===e&&this._sphericalEnd.radius===e||(this._spherical.radius=e,this._sphericalEnd.radius=e,this._needsUpdate=!0)}get azimuthAngle(){return this._spherical.theta}set azimuthAngle(e){this._spherical.theta===e&&this._sphericalEnd.theta===e||(this._spherical.theta=e,this._sphericalEnd.theta=e,this._needsUpdate=!0)}get polarAngle(){return this._spherical.phi}set polarAngle(e){this._spherical.phi===e&&this._sphericalEnd.phi===e||(this._spherical.phi=e,this._sphericalEnd.phi=e,this._needsUpdate=!0)}get boundaryEnclosesCamera(){return this._boundaryEnclosesCamera}set boundaryEnclosesCamera(e){this._boundaryEnclosesCamera=e,this._needsUpdate=!0}set interactiveArea(e){this._interactiveArea.width=Q(e.width,0,1),this._interactiveArea.height=Q(e.height,0,1),this._interactiveArea.x=Q(e.x,0,1-this._interactiveArea.width),this._interactiveArea.y=Q(e.y,0,1-this._interactiveArea.height)}addEventListener(e,n){super.addEventListener(e,n)}removeEventListener(e,n){super.removeEventListener(e,n)}rotate(e,n,t=!1){return this.rotateTo(this._sphericalEnd.theta+e,this._sphericalEnd.phi+n,t)}rotateAzimuthTo(e,n=!1){return this.rotateTo(e,this._sphericalEnd.phi,n)}rotatePolarTo(e,n=!1){return this.rotateTo(this._sphericalEnd.theta,e,n)}rotateTo(e,n,t=!1){this._isUserControllingRotate=!1;const a=Q(e,this.minAzimuthAngle,this.maxAzimuthAngle),s=Q(n,this.minPolarAngle,this.maxPolarAngle);this._sphericalEnd.theta=a,this._sphericalEnd.phi=s,this._sphericalEnd.makeSafe(),this._needsUpdate=!0,t||(this._spherical.theta=this._sphericalEnd.theta,this._spherical.phi=this._sphericalEnd.phi);const l=!t||N(this._spherical.theta,this._sphericalEnd.theta,this.restThreshold)&&N(this._spherical.phi,this._sphericalEnd.phi,this.restThreshold);return this._createOnRestPromise(l)}dolly(e,n=!1){return this.dollyTo(this._sphericalEnd.radius-e,n)}dollyTo(e,n=!1){return this._isUserControllingDolly=!1,this._lastDollyDirection=fe.NONE,this._changedDolly=0,this._dollyToNoClamp(Q(e,this.minDistance,this.maxDistance),n)}_dollyToNoClamp(e,n=!1){const t=this._sphericalEnd.radius;if(this.colliderMeshes.length>=1){const l=this._collisionTest(),f=N(l,this._spherical.radius);if(!(t>e)&&f)return Promise.resolve();this._sphericalEnd.radius=Math.min(e,l)}else this._sphericalEnd.radius=e;this._needsUpdate=!0,n||(this._spherical.radius=this._sphericalEnd.radius);const s=!n||N(this._spherical.radius,this._sphericalEnd.radius,this.restThreshold);return this._createOnRestPromise(s)}dollyInFixed(e,n=!1){this._targetEnd.add(this._getCameraDirection(ye).multiplyScalar(e)),n||this._target.copy(this._targetEnd);const t=!n||N(this._target.x,this._targetEnd.x,this.restThreshold)&&N(this._target.y,this._targetEnd.y,this.restThreshold)&&N(this._target.z,this._targetEnd.z,this.restThreshold);return this._createOnRestPromise(t)}zoom(e,n=!1){return this.zoomTo(this._zoomEnd+e,n)}zoomTo(e,n=!1){this._isUserControllingZoom=!1,this._zoomEnd=Q(e,this.minZoom,this.maxZoom),this._needsUpdate=!0,n||(this._zoom=this._zoomEnd);const t=!n||N(this._zoom,this._zoomEnd,this.restThreshold);return this._changedZoom=0,this._createOnRestPromise(t)}pan(e,n,t=!1){return console.warn("`pan` has been renamed to `truck`"),this.truck(e,n,t)}truck(e,n,t=!1){this._camera.updateMatrix(),J.setFromMatrixColumn(this._camera.matrix,0),ee.setFromMatrixColumn(this._camera.matrix,1),J.multiplyScalar(e),ee.multiplyScalar(-n);const a=z.copy(J).add(ee),s=O.copy(this._targetEnd).add(a);return this.moveTo(s.x,s.y,s.z,t)}forward(e,n=!1){z.setFromMatrixColumn(this._camera.matrix,0),z.crossVectors(this._camera.up,z),z.multiplyScalar(e);const t=O.copy(this._targetEnd).add(z);return this.moveTo(t.x,t.y,t.z,n)}elevate(e,n=!1){return z.copy(this._camera.up).multiplyScalar(e),this.moveTo(this._targetEnd.x+z.x,this._targetEnd.y+z.y,this._targetEnd.z+z.z,n)}moveTo(e,n,t,a=!1){this._isUserControllingTruck=!1;const s=z.set(e,n,t).sub(this._targetEnd);this._encloseToBoundary(this._targetEnd,s,this.boundaryFriction),this._needsUpdate=!0,a||this._target.copy(this._targetEnd);const l=!a||N(this._target.x,this._targetEnd.x,this.restThreshold)&&N(this._target.y,this._targetEnd.y,this.restThreshold)&&N(this._target.z,this._targetEnd.z,this.restThreshold);return this._createOnRestPromise(l)}lookInDirectionOf(e,n,t,a=!1){const f=z.set(e,n,t).sub(this._targetEnd).normalize().multiplyScalar(-this._sphericalEnd.radius).add(this._targetEnd);return this.setPosition(f.x,f.y,f.z,a)}fitToBox(e,n,{cover:t=!1,paddingLeft:a=0,paddingRight:s=0,paddingBottom:l=0,paddingTop:f=0}={}){const m=[],p=e.isBox3?me.copy(e):me.setFromObject(e);p.isEmpty()&&(console.warn("camera-controls: fitTo() cannot be used with an empty box. Aborting"),Promise.resolve());const P=$e(this._sphericalEnd.theta,je),h=$e(this._sphericalEnd.phi,je);m.push(this.rotateTo(P,h,n));const w=z.setFromSpherical(this._sphericalEnd).normalize(),_=tn.setFromUnitVectors(w,Ie),L=N(Math.abs(w.y),1);L&&_.multiply(Me.setFromAxisAngle(be,P)),_.multiply(this._yAxisUpSpaceInverse);const E=nn.makeEmpty();O.copy(p.min).applyQuaternion(_),E.expandByPoint(O),O.copy(p.min).setX(p.max.x).applyQuaternion(_),E.expandByPoint(O),O.copy(p.min).setY(p.max.y).applyQuaternion(_),E.expandByPoint(O),O.copy(p.max).setZ(p.min.z).applyQuaternion(_),E.expandByPoint(O),O.copy(p.min).setZ(p.max.z).applyQuaternion(_),E.expandByPoint(O),O.copy(p.max).setY(p.min.y).applyQuaternion(_),E.expandByPoint(O),O.copy(p.max).setX(p.min.x).applyQuaternion(_),E.expandByPoint(O),O.copy(p.max).applyQuaternion(_),E.expandByPoint(O),E.min.x-=a,E.min.y-=l,E.max.x+=s,E.max.y+=f,_.setFromUnitVectors(Ie,w),L&&_.premultiply(Me.invert()),_.premultiply(this._yAxisUpSpace);const R=E.getSize(z),c=E.getCenter(O).applyQuaternion(_);if(ae(this._camera)){const g=this.getDistanceToFitBox(R.x,R.y,R.z,t);m.push(this.moveTo(c.x,c.y,c.z,n)),m.push(this.dollyTo(g,n)),m.push(this.setFocalOffset(0,0,0,n))}else if(oe(this._camera)){const g=this._camera,v=g.right-g.left,y=g.top-g.bottom,b=t?Math.max(v/R.x,y/R.y):Math.min(v/R.x,y/R.y);m.push(this.moveTo(c.x,c.y,c.z,n)),m.push(this.zoomTo(b,n)),m.push(this.setFocalOffset(0,0,0,n))}return Promise.all(m)}fitToSphere(e,n){const t=[],s="isObject3D"in e?W.createBoundingSphere(e,Fe):Fe.copy(e);if(t.push(this.moveTo(s.center.x,s.center.y,s.center.z,n)),ae(this._camera)){const l=this.getDistanceToFitSphere(s.radius);t.push(this.dollyTo(l,n))}else if(oe(this._camera)){const l=this._camera.right-this._camera.left,f=this._camera.top-this._camera.bottom,m=2*s.radius,p=Math.min(l/m,f/m);t.push(this.zoomTo(p,n))}return t.push(this.setFocalOffset(0,0,0,n)),Promise.all(t)}setLookAt(e,n,t,a,s,l,f=!1){this._isUserControllingRotate=!1,this._isUserControllingDolly=!1,this._isUserControllingTruck=!1,this._lastDollyDirection=fe.NONE,this._changedDolly=0;const m=O.set(a,s,l),p=z.set(e,n,t);this._targetEnd.copy(m),this._sphericalEnd.setFromVector3(p.sub(m).applyQuaternion(this._yAxisUpSpace)),this.normalizeRotations(),this._needsUpdate=!0,f||(this._target.copy(this._targetEnd),this._spherical.copy(this._sphericalEnd));const P=!f||N(this._target.x,this._targetEnd.x,this.restThreshold)&&N(this._target.y,this._targetEnd.y,this.restThreshold)&&N(this._target.z,this._targetEnd.z,this.restThreshold)&&N(this._spherical.theta,this._sphericalEnd.theta,this.restThreshold)&&N(this._spherical.phi,this._sphericalEnd.phi,this.restThreshold)&&N(this._spherical.radius,this._sphericalEnd.radius,this.restThreshold);return this._createOnRestPromise(P)}lerpLookAt(e,n,t,a,s,l,f,m,p,P,h,w,_,L=!1){this._isUserControllingRotate=!1,this._isUserControllingDolly=!1,this._isUserControllingTruck=!1,this._lastDollyDirection=fe.NONE,this._changedDolly=0;const E=z.set(a,s,l),R=O.set(e,n,t);$.setFromVector3(R.sub(E).applyQuaternion(this._yAxisUpSpace));const c=de.set(P,h,w),g=O.set(f,m,p);xe.setFromVector3(g.sub(c).applyQuaternion(this._yAxisUpSpace)),this._targetEnd.copy(E.lerp(c,_));const v=xe.theta-$.theta,y=xe.phi-$.phi,b=xe.radius-$.radius;this._sphericalEnd.set($.radius+b*_,$.phi+y*_,$.theta+v*_),this.normalizeRotations(),this._needsUpdate=!0,L||(this._target.copy(this._targetEnd),this._spherical.copy(this._sphericalEnd));const T=!L||N(this._target.x,this._targetEnd.x,this.restThreshold)&&N(this._target.y,this._targetEnd.y,this.restThreshold)&&N(this._target.z,this._targetEnd.z,this.restThreshold)&&N(this._spherical.theta,this._sphericalEnd.theta,this.restThreshold)&&N(this._spherical.phi,this._sphericalEnd.phi,this.restThreshold)&&N(this._spherical.radius,this._sphericalEnd.radius,this.restThreshold);return this._createOnRestPromise(T)}setPosition(e,n,t,a=!1){return this.setLookAt(e,n,t,this._targetEnd.x,this._targetEnd.y,this._targetEnd.z,a)}setTarget(e,n,t,a=!1){const s=this.getPosition(z),l=this.setLookAt(s.x,s.y,s.z,e,n,t,a);return this._sphericalEnd.phi=Q(this._sphericalEnd.phi,this.minPolarAngle,this.maxPolarAngle),l}setFocalOffset(e,n,t,a=!1){this._isUserControllingOffset=!1,this._focalOffsetEnd.set(e,n,t),this._needsUpdate=!0,a||this._focalOffset.copy(this._focalOffsetEnd);const s=!a||N(this._focalOffset.x,this._focalOffsetEnd.x,this.restThreshold)&&N(this._focalOffset.y,this._focalOffsetEnd.y,this.restThreshold)&&N(this._focalOffset.z,this._focalOffsetEnd.z,this.restThreshold);return this._createOnRestPromise(s)}setOrbitPoint(e,n,t){this._camera.updateMatrixWorld(),J.setFromMatrixColumn(this._camera.matrixWorldInverse,0),ee.setFromMatrixColumn(this._camera.matrixWorldInverse,1),se.setFromMatrixColumn(this._camera.matrixWorldInverse,2);const a=z.set(e,n,t),s=a.distanceTo(this._camera.position),l=a.sub(this._camera.position);J.multiplyScalar(l.x),ee.multiplyScalar(l.y),se.multiplyScalar(l.z),z.copy(J).add(ee).add(se),z.z=z.z+s,this.dollyTo(s,!1),this.setFocalOffset(-z.x,z.y,-z.z,!1),this.moveTo(e,n,t,!1)}setBoundary(e){if(!e){this._boundary.min.set(-1/0,-1/0,-1/0),this._boundary.max.set(1/0,1/0,1/0),this._needsUpdate=!0;return}this._boundary.copy(e),this._boundary.clampPoint(this._targetEnd,this._targetEnd),this._needsUpdate=!0}setViewport(e,n,t,a){if(e===null){this._viewport=null;return}this._viewport=this._viewport||new C.Vector4,typeof e=="number"?this._viewport.set(e,n,t,a):this._viewport.copy(e)}getDistanceToFitBox(e,n,t,a=!1){if(Le(this._camera,"getDistanceToFitBox"))return this._spherical.radius;const s=e/n,l=this._camera.getEffectiveFOV()*pe,f=this._camera.aspect;return((a?s>f:s<f)?n:e/f)*.5/Math.tan(l*.5)+t*.5}getDistanceToFitSphere(e){if(Le(this._camera,"getDistanceToFitSphere"))return this._spherical.radius;const n=this._camera.getEffectiveFOV()*pe,t=Math.atan(Math.tan(n*.5)*this._camera.aspect)*2,a=1<this._camera.aspect?n:t;return e/Math.sin(a*.5)}getTarget(e,n=!0){return(e&&e.isVector3?e:new C.Vector3).copy(n?this._targetEnd:this._target)}getPosition(e,n=!0){return(e&&e.isVector3?e:new C.Vector3).setFromSpherical(n?this._sphericalEnd:this._spherical).applyQuaternion(this._yAxisUpSpaceInverse).add(n?this._targetEnd:this._target)}getSpherical(e,n=!0){return(e||new C.Spherical).copy(n?this._sphericalEnd:this._spherical)}getFocalOffset(e,n=!0){return(e&&e.isVector3?e:new C.Vector3).copy(n?this._focalOffsetEnd:this._focalOffset)}normalizeRotations(){this._sphericalEnd.theta=this._sphericalEnd.theta%ue,this._sphericalEnd.theta<0&&(this._sphericalEnd.theta+=ue),this._spherical.theta+=ue*Math.round((this._sphericalEnd.theta-this._spherical.theta)/ue)}stop(){this._focalOffset.copy(this._focalOffsetEnd),this._target.copy(this._targetEnd),this._spherical.copy(this._sphericalEnd),this._zoom=this._zoomEnd}reset(e=!1){if(!N(this._camera.up.x,this._cameraUp0.x)||!N(this._camera.up.y,this._cameraUp0.y)||!N(this._camera.up.z,this._cameraUp0.z)){this._camera.up.copy(this._cameraUp0);const t=this.getPosition(z);this.updateCameraUp(),this.setPosition(t.x,t.y,t.z)}const n=[this.setLookAt(this._position0.x,this._position0.y,this._position0.z,this._target0.x,this._target0.y,this._target0.z,e),this.setFocalOffset(this._focalOffset0.x,this._focalOffset0.y,this._focalOffset0.z,e),this.zoomTo(this._zoom0,e)];return Promise.all(n)}saveState(){this._cameraUp0.copy(this._camera.up),this.getTarget(this._target0),this.getPosition(this._position0),this._zoom0=this._zoom,this._focalOffset0.copy(this._focalOffset)}updateCameraUp(){this._yAxisUpSpace.setFromUnitVectors(this._camera.up,be),this._yAxisUpSpaceInverse.copy(this._yAxisUpSpace).invert()}applyCameraUp(){const e=z.subVectors(this._target,this._camera.position).normalize(),n=O.crossVectors(e,this._camera.up);this._camera.up.crossVectors(n,e).normalize(),this._camera.updateMatrixWorld();const t=this.getPosition(z);this.updateCameraUp(),this.setPosition(t.x,t.y,t.z)}update(e){const n=this._sphericalEnd.theta-this._spherical.theta,t=this._sphericalEnd.phi-this._spherical.phi,a=this._sphericalEnd.radius-this._spherical.radius,s=Je.subVectors(this._targetEnd,this._target),l=en.subVectors(this._focalOffsetEnd,this._focalOffset),f=this._zoomEnd-this._zoom;if(I(n))this._thetaVelocity.value=0,this._spherical.theta=this._sphericalEnd.theta;else{const h=this._isUserControllingRotate?this.draggingSmoothTime:this.smoothTime;this._spherical.theta=we(this._spherical.theta,this._sphericalEnd.theta,this._thetaVelocity,h,1/0,e),this._needsUpdate=!0}if(I(t))this._phiVelocity.value=0,this._spherical.phi=this._sphericalEnd.phi;else{const h=this._isUserControllingRotate?this.draggingSmoothTime:this.smoothTime;this._spherical.phi=we(this._spherical.phi,this._sphericalEnd.phi,this._phiVelocity,h,1/0,e),this._needsUpdate=!0}if(I(a))this._radiusVelocity.value=0,this._spherical.radius=this._sphericalEnd.radius;else{const h=this._isUserControllingDolly?this.draggingSmoothTime:this.smoothTime;this._spherical.radius=we(this._spherical.radius,this._sphericalEnd.radius,this._radiusVelocity,h,this.maxSpeed,e),this._needsUpdate=!0}if(I(s.x)&&I(s.y)&&I(s.z))this._targetVelocity.set(0,0,0),this._target.copy(this._targetEnd);else{const h=this._isUserControllingTruck?this.draggingSmoothTime:this.smoothTime;Ke(this._target,this._targetEnd,this._targetVelocity,h,this.maxSpeed,e,this._target),this._needsUpdate=!0}if(I(l.x)&&I(l.y)&&I(l.z))this._focalOffsetVelocity.set(0,0,0),this._focalOffset.copy(this._focalOffsetEnd);else{const h=this._isUserControllingOffset?this.draggingSmoothTime:this.smoothTime;Ke(this._focalOffset,this._focalOffsetEnd,this._focalOffsetVelocity,h,this.maxSpeed,e,this._focalOffset),this._needsUpdate=!0}if(I(f))this._zoomVelocity.value=0,this._zoom=this._zoomEnd;else{const h=this._isUserControllingZoom?this.draggingSmoothTime:this.smoothTime;this._zoom=we(this._zoom,this._zoomEnd,this._zoomVelocity,h,1/0,e)}if(this.dollyToCursor){if(ae(this._camera)&&this._changedDolly!==0){const h=this._spherical.radius-this._lastDistance,w=this._camera,_=this._getCameraDirection(ye),L=z.copy(_).cross(w.up).normalize();L.lengthSq()===0&&(L.x=1);const E=O.crossVectors(L,_),R=this._sphericalEnd.radius*Math.tan(w.getEffectiveFOV()*pe*.5),g=(this._sphericalEnd.radius-h-this._sphericalEnd.radius)/this._sphericalEnd.radius,v=de.copy(this._targetEnd).add(L.multiplyScalar(this._dollyControlCoord.x*R*w.aspect)).add(E.multiplyScalar(this._dollyControlCoord.y*R)),y=z.copy(this._targetEnd).lerp(v,g),b=this._lastDollyDirection===fe.IN&&this._spherical.radius<=this.minDistance,T=this._lastDollyDirection===fe.OUT&&this.maxDistance<=this._spherical.radius;if(this.infinityDolly&&(b||T)){this._sphericalEnd.radius-=h,this._spherical.radius-=h;const B=O.copy(_).multiplyScalar(-h);y.add(B)}this._boundary.clampPoint(y,y);const A=O.subVectors(y,this._targetEnd);this._targetEnd.copy(y),this._target.add(A),this._changedDolly-=h,I(this._changedDolly)&&(this._changedDolly=0)}else if(oe(this._camera)&&this._changedZoom!==0){const h=this._zoom-this._lastZoom,w=this._camera,_=z.set(this._dollyControlCoord.x,this._dollyControlCoord.y,(w.near+w.far)/(w.near-w.far)).unproject(w),L=O.set(0,0,-1).applyQuaternion(w.quaternion),E=de.copy(_).add(L.multiplyScalar(-_.dot(w.up))),c=-(this._zoom-h-this._zoom)/this._zoom,g=this._getCameraDirection(ye),v=this._targetEnd.dot(g),y=z.copy(this._targetEnd).lerp(E,c),b=y.dot(g),T=g.multiplyScalar(b-v);y.sub(T),this._boundary.clampPoint(y,y);const A=O.subVectors(y,this._targetEnd);this._targetEnd.copy(y),this._target.add(A),this._changedZoom-=h,I(this._changedZoom)&&(this._changedZoom=0)}}this._camera.zoom!==this._zoom&&(this._camera.zoom=this._zoom,this._camera.updateProjectionMatrix(),this._updateNearPlaneCorners(),this._needsUpdate=!0),this._dragNeedsUpdate=!0;const m=this._collisionTest();this._spherical.radius=Math.min(this._spherical.radius,m),this._spherical.makeSafe(),this._camera.position.setFromSpherical(this._spherical).applyQuaternion(this._yAxisUpSpaceInverse).add(this._target),this._camera.lookAt(this._target),(!I(this._focalOffset.x)||!I(this._focalOffset.y)||!I(this._focalOffset.z))&&(J.setFromMatrixColumn(this._camera.matrix,0),ee.setFromMatrixColumn(this._camera.matrix,1),se.setFromMatrixColumn(this._camera.matrix,2),J.multiplyScalar(this._focalOffset.x),ee.multiplyScalar(-this._focalOffset.y),se.multiplyScalar(this._focalOffset.z),z.copy(J).add(ee).add(se),this._camera.position.add(z),this._camera.updateMatrixWorld()),this._boundaryEnclosesCamera&&this._encloseToBoundary(this._camera.position.copy(this._target),z.setFromSpherical(this._spherical).applyQuaternion(this._yAxisUpSpaceInverse),1);const P=this._needsUpdate;return P&&!this._updatedLastTime?(this._hasRested=!1,this.dispatchEvent({type:"wake"}),this.dispatchEvent({type:"update"})):P?(this.dispatchEvent({type:"update"}),I(n,this.restThreshold)&&I(t,this.restThreshold)&&I(a,this.restThreshold)&&I(s.x,this.restThreshold)&&I(s.y,this.restThreshold)&&I(s.z,this.restThreshold)&&I(l.x,this.restThreshold)&&I(l.y,this.restThreshold)&&I(l.z,this.restThreshold)&&I(f,this.restThreshold)&&!this._hasRested&&(this._hasRested=!0,this.dispatchEvent({type:"rest"}))):!P&&this._updatedLastTime&&this.dispatchEvent({type:"sleep"}),this._lastDistance=this._spherical.radius,this._lastZoom=this._zoom,this._updatedLastTime=P,this._needsUpdate=!1,P}toJSON(){return JSON.stringify({enabled:this._enabled,minDistance:this.minDistance,maxDistance:he(this.maxDistance),minZoom:this.minZoom,maxZoom:he(this.maxZoom),minPolarAngle:this.minPolarAngle,maxPolarAngle:he(this.maxPolarAngle),minAzimuthAngle:he(this.minAzimuthAngle),maxAzimuthAngle:he(this.maxAzimuthAngle),smoothTime:this.smoothTime,draggingSmoothTime:this.draggingSmoothTime,dollySpeed:this.dollySpeed,truckSpeed:this.truckSpeed,dollyToCursor:this.dollyToCursor,target:this._targetEnd.toArray(),position:z.setFromSpherical(this._sphericalEnd).add(this._targetEnd).toArray(),zoom:this._zoomEnd,focalOffset:this._focalOffsetEnd.toArray(),target0:this._target0.toArray(),position0:this._position0.toArray(),zoom0:this._zoom0,focalOffset0:this._focalOffset0.toArray()})}fromJSON(e,n=!1){const t=JSON.parse(e);this.enabled=t.enabled,this.minDistance=t.minDistance,this.maxDistance=_e(t.maxDistance),this.minZoom=t.minZoom,this.maxZoom=_e(t.maxZoom),this.minPolarAngle=t.minPolarAngle,this.maxPolarAngle=_e(t.maxPolarAngle),this.minAzimuthAngle=_e(t.minAzimuthAngle),this.maxAzimuthAngle=_e(t.maxAzimuthAngle),this.smoothTime=t.smoothTime,this.draggingSmoothTime=t.draggingSmoothTime,this.dollySpeed=t.dollySpeed,this.truckSpeed=t.truckSpeed,this.dollyToCursor=t.dollyToCursor,this._target0.fromArray(t.target0),this._position0.fromArray(t.position0),this._zoom0=t.zoom0,this._focalOffset0.fromArray(t.focalOffset0),this.moveTo(t.target[0],t.target[1],t.target[2],n),$.setFromVector3(z.fromArray(t.position).sub(this._targetEnd).applyQuaternion(this._yAxisUpSpace)),this.rotateTo($.theta,$.phi,n),this.dollyTo($.radius,n),this.zoomTo(t.zoom,n),this.setFocalOffset(t.focalOffset[0],t.focalOffset[1],t.focalOffset[2],n),this._needsUpdate=!0}connect(e){if(this._domElement){console.warn("camera-controls is already connected.");return}e.setAttribute("data-camera-controls-version",Yn),this._addAllEventListeners(e),this._getClientRect(this._elementRect)}disconnect(){this.cancel(),this._removeAllEventListeners(),this._domElement&&(this._domElement.removeAttribute("data-camera-controls-version"),this._domElement=void 0)}dispose(){this.removeAllEventListeners(),this.disconnect()}_getTargetDirection(e){return e.setFromSpherical(this._spherical).divideScalar(this._spherical.radius).applyQuaternion(this._yAxisUpSpaceInverse)}_getCameraDirection(e){return this._getTargetDirection(e).negate()}_findPointerById(e){return this._activePointers.find(n=>n.pointerId===e)}_findPointerByMouseButton(e){return this._activePointers.find(n=>n.mouseButton===e)}_disposePointer(e){this._activePointers.splice(this._activePointers.indexOf(e),1)}_encloseToBoundary(e,n,t){const a=n.lengthSq();if(a===0)return e;const s=O.copy(n).add(e),f=this._boundary.clampPoint(s,de).sub(s),m=f.lengthSq();if(m===0)return e.add(n);if(m===a)return e;if(t===0)return e.add(n).add(f);{const p=1+t*m/n.dot(f);return e.add(O.copy(n).multiplyScalar(p)).add(f.multiplyScalar(1-t))}}_updateNearPlaneCorners(){if(ae(this._camera)){const e=this._camera,n=e.near,t=e.getEffectiveFOV()*pe,a=Math.tan(t*.5)*n,s=a*e.aspect;this._nearPlaneCorners[0].set(-s,-a,0),this._nearPlaneCorners[1].set(s,-a,0),this._nearPlaneCorners[2].set(s,a,0),this._nearPlaneCorners[3].set(-s,a,0)}else if(oe(this._camera)){const e=this._camera,n=1/e.zoom,t=e.left*n,a=e.right*n,s=e.top*n,l=e.bottom*n;this._nearPlaneCorners[0].set(t,s,0),this._nearPlaneCorners[1].set(a,s,0),this._nearPlaneCorners[2].set(a,l,0),this._nearPlaneCorners[3].set(t,l,0)}}_collisionTest(){let e=1/0;if(!(this.colliderMeshes.length>=1)||Le(this._camera,"_collisionTest"))return e;const t=this._getTargetDirection(ye);He.lookAt(Qe,t,this._camera.up);for(let a=0;a<4;a++){const s=O.copy(this._nearPlaneCorners[a]);s.applyMatrix4(He);const l=de.addVectors(this._target,s);Se.set(l,t),Se.far=this._spherical.radius+1;const f=Se.intersectObjects(this.colliderMeshes);f.length!==0&&f[0].distance<e&&(e=f[0].distance)}return e}_getClientRect(e){if(!this._domElement)return;const n=this._domElement.getBoundingClientRect();return e.x=n.left,e.y=n.top,this._viewport?(e.x+=this._viewport.x,e.y+=n.height-this._viewport.w-this._viewport.y,e.width=this._viewport.z,e.height=this._viewport.w):(e.width=n.width,e.height=n.height),e}_createOnRestPromise(e){return e?Promise.resolve():(this._hasRested=!1,this.dispatchEvent({type:"transitionstart"}),new Promise(n=>{const t=()=>{this.removeEventListener("rest",t),n()};this.addEventListener("rest",t)}))}_addAllEventListeners(e){}_removeAllEventListeners(){}get dampingFactor(){return console.warn(".dampingFactor has been deprecated. use smoothTime (in seconds) instead."),0}set dampingFactor(e){console.warn(".dampingFactor has been deprecated. use smoothTime (in seconds) instead.")}get draggingDampingFactor(){return console.warn(".draggingDampingFactor has been deprecated. use draggingSmoothTime (in seconds) instead."),0}set draggingDampingFactor(e){console.warn(".draggingDampingFactor has been deprecated. use draggingSmoothTime (in seconds) instead.")}static createBoundingSphere(e,n=new C.Sphere){const t=n,a=t.center;me.makeEmpty(),e.traverseVisible(l=>{l.isMesh&&me.expandByObject(l)}),me.getCenter(a);let s=0;return e.traverseVisible(l=>{if(!l.isMesh)return;const f=l;if(!f.geometry)return;const m=f.geometry.clone();m.applyMatrix4(f.matrixWorld);const P=m.attributes.position;for(let h=0,w=P.count;h<w;h++)z.fromBufferAttribute(P,h),s=Math.max(s,a.distanceToSquared(z))}),t.radius=Math.sqrt(s),t}}var rn=Object.defineProperty,Z=(o,e)=>{let n={};for(var t in o)rn(n,t,{get:o[t],enumerable:!0});return rn(n,Symbol.toStringTag,{value:"Module"}),n};const u={type:"plane",control:"props",urlString:"",animate:!0,uTime:0,uSpeed:.4,uStrength:4,uDensity:1.3,uFrequency:5.5,uAmplitude:1,range:!1,rangeStart:0,rangeEnd:40,loop:!1,loopDuration:8,positionX:-1.4,positionY:0,positionZ:0,rotationX:0,rotationY:10,rotationZ:50,color1:"#ff5005",color2:"#dbba95",color3:"#d0bce1",reflection:.1,wireframe:!1,shader:"defaults",cAzimuthAngle:180,cPolarAngle:90,cDistance:3.6,cameraZoom:1,fov:45,lightType:"3d",brightness:1.2,envPreset:"city",grain:!1,grainBlending:1,toggleAxis:!1,zoomOut:!1,hoverState:"",smoothTime:.14,enableTransition:!1,enableCameraControls:!1,enableCameraUpdate:!1,pixelDensity:2,preserveDrawingBuffer:!1,powerPreference:void 0,envBasePath:"https://ruucm.github.io/shadergradient/ui@0.0.0/assets/hdr/"},Ye={halo:{title:"Halo",color:"white",props:{type:"plane",animate:"on",uTime:0,uSpeed:.4,uStrength:4,uDensity:1.3,uFrequency:5.5,uAmplitude:1,range:"disabled",rangeStart:0,rangeEnd:40,positionX:-1.4,positionY:0,positionZ:0,rotationX:0,rotationY:10,rotationZ:50,color1:"#ff5005",color2:"#dbba95",color3:"#d0bce1",reflection:.1,cAzimuthAngle:180,cDistance:3.6,cPolarAngle:90,cameraZoom:1,lightType:"3d",brightness:1.2,envPreset:"city",grain:"on"}},pensive:{title:"Pensive",color:"white",props:{type:"sphere",animate:"on",uTime:0,uSpeed:.3,uStrength:.4,uDensity:.8,uFrequency:5.5,uAmplitude:7,positionX:0,positionY:0,positionZ:0,rotationX:0,rotationY:0,rotationZ:140,color1:"#809bd6",color2:"#910aff",color3:"#af38ff",reflection:.5,cAzimuthAngle:250,cDistance:1.5,cPolarAngle:140,cameraZoom:12.5,lightType:"3d",brightness:1.5,envPreset:"city",grain:"on"}},mint:{title:"Mint",color:"white",props:{type:"waterPlane",animate:"on",uTime:0,uSpeed:.2,uStrength:3.4,uDensity:1.2,uFrequency:0,uAmplitude:0,positionX:0,positionY:.9,positionZ:-.3,rotationX:45,rotationY:0,rotationZ:0,color1:"#94ffd1",color2:"#6bf5ff",color3:"#ffffff",reflection:.1,cAzimuthAngle:170,cDistance:4.4,cPolarAngle:70,cameraZoom:1,lightType:"3d",brightness:1.2,envPreset:"city",grain:"off"}},interstella:{title:"Interstella",color:"white",props:{type:"sphere",animate:"on",uTime:0,uSpeed:.3,uStrength:.3,uDensity:.8,uFrequency:5.5,uAmplitude:3.2,positionX:-.1,positionY:0,positionZ:0,rotationX:0,rotationY:130,rotationZ:70,color1:"#73bfc4",color2:"#ff810a",color3:"#8da0ce",reflection:.4,cAzimuthAngle:270,cDistance:.5,cPolarAngle:180,cameraZoom:15.1,lightType:"env",brightness:.8,envPreset:"city",grain:"on"}},nightyNight:{title:"Nighty Night",color:"white",props:{type:"waterPlane",animate:"on",uTime:8,uSpeed:.3,uStrength:1.5,uDensity:1.5,uFrequency:0,uAmplitude:0,positionX:0,positionY:0,positionZ:0,rotationX:50,rotationY:0,rotationZ:-60,color1:"#606080",color2:"#8d7dca",color3:"#212121",reflection:.1,cAzimuthAngle:180,cDistance:2.8,cPolarAngle:80,cameraZoom:9.1,lightType:"3d",brightness:1,envPreset:"city",grain:"on"}},violaOrientalis:{title:"Viola",color:"white",props:{type:"sphere",animate:"on",uTime:0,uSpeed:.1,uStrength:1,uDensity:1.1,uFrequency:5.5,uAmplitude:1.4,positionX:0,positionY:0,positionZ:0,rotationX:0,rotationY:0,rotationZ:0,color1:"#ffffff",color2:"#ffbb00",color3:"#0700ff",reflection:.1,cAzimuthAngle:0,cDistance:7.1,cPolarAngle:140,cameraZoom:17.3,lightType:"3d",brightness:1.1,envPreset:"city",grain:"off"}},universe:{title:"Universe",color:"white",props:{type:"waterPlane",animate:"on",uTime:.2,uSpeed:.1,uStrength:2.4,uDensity:1.1,uFrequency:5.5,uAmplitude:0,positionX:-.5,positionY:.1,positionZ:0,rotationX:0,rotationY:0,rotationZ:235,color1:"#5606ff",color2:"#fe8989",color3:"#000000",reflection:.1,cAzimuthAngle:180,cDistance:3.9,cPolarAngle:115,cameraZoom:1,lightType:"3d",brightness:1.1,envPreset:"city",grain:"off"}},sunset:{title:"Sunset",color:"white",props:{type:"sphere",animate:"on",uTime:0,uSpeed:.1,uStrength:.4,uDensity:1.1,uFrequency:5.5,uAmplitude:1.4,positionX:0,positionY:-.15,positionZ:0,rotationX:0,rotationY:0,rotationZ:0,color1:"#ff7a33",color2:"#33a0ff",color3:"#ffc53d",reflection:.1,cAzimuthAngle:60,cDistance:7.1,cPolarAngle:90,cameraZoom:15.3,lightType:"3d",brightness:1.5,envPreset:"dawn",grain:"off"}},mandarin:{title:"Mandarin",color:"white",props:{type:"waterPlane",animate:"on",uTime:.2,uSpeed:.2,uStrength:3,uDensity:1.8,uFrequency:5.5,uAmplitude:0,positionX:0,positionY:-2.1,positionZ:0,rotationX:0,rotationY:0,rotationZ:225,color1:"#ff6a1a",color2:"#c73c00",color3:"#fd4912",reflection:.1,cAzimuthAngle:180,cDistance:2.4,cPolarAngle:95,cameraZoom:1,lightType:"3d",brightness:1.2,envPreset:"city",grain:"off"}},cottonCandy:{title:"Cotton Candy",color:"white",props:{type:"waterPlane",animate:"on",uTime:.2,uSpeed:.3,uStrength:3,uDensity:1,uFrequency:5.5,uAmplitude:0,positionX:0,positionY:1.8,positionZ:0,rotationX:0,rotationY:0,rotationZ:-90,color1:"#ebedff",color2:"#f3f2f8",color3:"#dbf8ff",reflection:.1,cAzimuthAngle:180,cDistance:2.9,cPolarAngle:120,cameraZoom:1,lightType:"3d",brightness:1.2,envPreset:"city",grain:"off"}}};function Pe(o,e,n){return Math.min(n,Math.max(e,o))}function Ce(o){return o*Math.PI/180}function on(o){return o*180/Math.PI}function ne(o){const e=o.trim().replace("#",""),n=e.length===3?e.split("").map(t=>t+t).join(""):e;return[parseInt(n.slice(0,2),16)/255,parseInt(n.slice(2,4),16)/255,parseInt(n.slice(4,6),16)/255]}function Bn([o,e,n]){return`#${[o,e,n].map(t=>Math.round(Pe(t,0,1)*255).toString(16).padStart(2,"0")).join("")}`}function ke(o,e){return o===void 0?e:typeof o=="boolean"?o:o==="on"}function Zn(o,e){return o===void 0?e:typeof o=="boolean"?o:o==="enabled"}function x(o,e){if(typeof o=="number"&&Number.isFinite(o))return o;if(typeof o=="string"&&o.trim()){const n=Number(o);if(Number.isFinite(n))return n}return e}function X(o,e){if(typeof o=="boolean")return o;if(typeof o=="string"){if(o==="true"||o==="on"||o==="enabled")return!0;if(o==="false"||o==="off"||o==="disabled")return!1}return e}function Ae(o,e,n,t){const a=Math.max(1e-4,1/Math.max(.001,n));return o+(e-o)*(1-Math.exp(-a*t))}function qn(o,e,n,t){return[Ae(o[0],e[0],n,t),Ae(o[1],e[1],n,t),Ae(o[2],e[2],n,t)]}function Gn(o){return o.replace("http://localhost:3001/customize","").replace("https://shadergradient.co/customize","").replace("https://www.shadergradient.co/customize","")}const Wn=new Set(Object.keys(Ye)),dn=new Set(["defaults","positionMix","cosmic","glass"]);function Xn(o){const e=Gn(o).trim(),n=e.startsWith("?")?e.slice(1):e.split("?")[1]??e,t=new URLSearchParams(n),a={};for(const[s,l]of t.entries()){if(l==="true"||l==="false"){a[s]=l==="true";continue}const f=Number(l);a[s]=Number.isFinite(f)&&l.trim()!==""?f:l}return a}function jn(o){const e=Xn(o),n=t=>Object.prototype.hasOwnProperty.call(e,t);return{preset:n("preset")&&typeof e.preset=="string"&&Wn.has(e.preset)?e.preset:void 0,type:n("type")&&typeof e.type=="string"?e.type:void 0,animate:n("animate")?X(e.animate,u.animate):void 0,uTime:n("uTime")?x(e.uTime,u.uTime):void 0,uSpeed:n("uSpeed")?x(e.uSpeed,u.uSpeed):void 0,uStrength:n("uStrength")?x(e.uStrength,u.uStrength):void 0,uDensity:n("uDensity")?x(e.uDensity,u.uDensity):void 0,uFrequency:n("uFrequency")?x(e.uFrequency,u.uFrequency):void 0,uAmplitude:n("uAmplitude")?x(e.uAmplitude,u.uAmplitude):void 0,range:n("range")?X(e.range,u.range):void 0,rangeStart:n("rangeStart")?x(e.rangeStart,u.rangeStart):void 0,rangeEnd:n("rangeEnd")?x(e.rangeEnd,u.rangeEnd):void 0,loop:n("loop")?X(e.loop,u.loop):void 0,loopDuration:n("loopDuration")?x(e.loopDuration,u.loopDuration):void 0,positionX:n("positionX")?x(e.positionX,u.positionX):void 0,positionY:n("positionY")?x(e.positionY,u.positionY):void 0,positionZ:n("positionZ")?x(e.positionZ,u.positionZ):void 0,rotationX:n("rotationX")?x(e.rotationX,u.rotationX):void 0,rotationY:n("rotationY")?x(e.rotationY,u.rotationY):void 0,rotationZ:n("rotationZ")?x(e.rotationZ,u.rotationZ):void 0,color1:n("color1")&&typeof e.color1=="string"?e.color1:void 0,color2:n("color2")&&typeof e.color2=="string"?e.color2:void 0,color3:n("color3")&&typeof e.color3=="string"?e.color3:void 0,reflection:n("reflection")?x(e.reflection,u.reflection):void 0,wireframe:n("wireframe")?X(e.wireframe,u.wireframe):void 0,shader:n("shader")&&typeof e.shader=="string"&&dn.has(e.shader)?e.shader:void 0,cAzimuthAngle:n("cAzimuthAngle")?x(e.cAzimuthAngle,u.cAzimuthAngle):void 0,cPolarAngle:n("cPolarAngle")?x(e.cPolarAngle,u.cPolarAngle):void 0,cDistance:n("cDistance")?x(e.cDistance,u.cDistance):void 0,cameraZoom:n("cameraZoom")?x(e.cameraZoom,u.cameraZoom):void 0,lightType:n("lightType")&&typeof e.lightType=="string"?e.lightType:void 0,brightness:n("brightness")?x(e.brightness,u.brightness):void 0,envPreset:n("envPreset")&&typeof e.envPreset=="string"?e.envPreset:void 0,grain:n("grain")?X(e.grain,u.grain):void 0,grainBlending:n("grainBlending")?x(e.grainBlending,u.grainBlending):void 0,toggleAxis:n("toggleAxis")?X(e.toggleAxis,u.toggleAxis):void 0,pixelDensity:n("pixelDensity")?x(e.pixelDensity,u.pixelDensity):void 0,fov:n("fov")?x(e.fov,u.fov):void 0,preserveDrawingBuffer:n("preserveDrawingBuffer")?X(e.preserveDrawingBuffer,u.preserveDrawingBuffer):void 0,powerPreference:n("powerPreference")&&typeof e.powerPreference=="string"?e.powerPreference:void 0}}function an(o={}){const e=o.control==="query"&&o.urlString?jn(o.urlString):void 0,n={...((e==null?void 0:e.preset)??o.preset)&&Ye[(e==null?void 0:e.preset)??o.preset]?Ye[(e==null?void 0:e.preset)??o.preset].props:void 0,...o,...e};return{preset:n.preset,type:n.type??u.type,control:n.control??u.control,urlString:n.urlString??u.urlString,animate:ke(n.animate,u.animate),uTime:x(n.uTime,u.uTime),uSpeed:x(n.uSpeed??n.speed,u.uSpeed),uStrength:x(n.uStrength??n.strength,u.uStrength),uDensity:x(n.uDensity??n.density,u.uDensity),uFrequency:x(n.uFrequency??n.frequency,u.uFrequency),uAmplitude:x(n.uAmplitude??n.amplitude,u.uAmplitude),range:Zn(n.range,u.range),rangeStart:x(n.rangeStart,u.rangeStart),rangeEnd:x(n.rangeEnd,u.rangeEnd),loop:ke(n.loop,u.loop),loopDuration:Math.max(.1,x(n.loopDuration,u.loopDuration)),positionX:x(n.positionX,u.positionX),positionY:x(n.positionY,u.positionY),positionZ:x(n.positionZ,u.positionZ),rotationX:x(n.rotationX,u.rotationX),rotationY:x(n.rotationY,u.rotationY),rotationZ:x(n.rotationZ,u.rotationZ),color1:n.color1??u.color1,color2:n.color2??u.color2,color3:n.color3??u.color3,reflection:Pe(x(n.reflection,u.reflection),0,1),wireframe:X(n.wireframe,u.wireframe),shader:typeof n.shader=="string"&&dn.has(n.shader)?n.shader:u.shader,cAzimuthAngle:x(n.cAzimuthAngle??n.cameraAzimuth,u.cAzimuthAngle),cPolarAngle:x(n.cPolarAngle??n.cameraPolar,u.cPolarAngle),cDistance:Math.max(.1,x(n.cDistance??n.cameraDistance,u.cDistance)),cameraZoom:Math.max(.1,x(n.cameraZoom,u.cameraZoom)),fov:Pe(x(n.fov,u.fov),10,120),lightType:n.lightType??u.lightType,brightness:Math.max(0,x(n.brightness,u.brightness)),envPreset:n.envPreset??u.envPreset,grain:ke(n.grain,u.grain),grainBlending:Pe(x(n.grainBlending,u.grainBlending),0,1),toggleAxis:X(n.toggleAxis??n.axesHelper,u.toggleAxis),zoomOut:X(n.zoomOut,u.zoomOut),hoverState:typeof n.hoverState=="string"?n.hoverState:u.hoverState,smoothTime:Math.max(.01,x(n.smoothTime,u.smoothTime)),enableTransition:X(n.enableTransition,u.enableTransition),enableCameraControls:X(n.enableCameraControls,u.enableCameraControls),enableCameraUpdate:X(n.enableCameraUpdate,u.enableCameraUpdate),pixelDensity:Pe(x(n.pixelDensity??n.pixelRatio,u.pixelDensity),.5,3),preserveDrawingBuffer:X(n.preserveDrawingBuffer,u.preserveDrawingBuffer),powerPreference:n.powerPreference??u.powerPreference,envBasePath:typeof n.envBasePath=="string"?n.envBasePath:u.envBasePath,onCameraUpdate:n.onCameraUpdate}}var $n=`// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- start here ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);

  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

varying vec2 vUv;

uniform float uTime;
uniform float uSpeed;
uniform float uLoop;
uniform float uLoopDuration;

uniform float uLoadingTime;

uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  vUv = uv;

  float t = uTime * uSpeed;
  
  // For seamless loops, sample noise using 4D-like circular interpolation
  vec3 noisePos = 0.43 * position * uNoiseDensity;
  float distortion;
  
  if (uLoop > 0.5) {
    // Create truly dynamic seamless loop using 4D noise simulation
    // Loop progress only depends on time and duration, not speed
    float loopProgress = uTime / uLoopDuration;
    float angle = loopProgress * 6.28318530718; // 2*PI
    
    // Radius scales with speed to maintain consistent visual speed
    // Larger radius = more distance traveled = faster perceived motion
    float radius = 5.0 * uSpeed;
    
    // Sample 4 noise values at cardinal points around the circle
    vec3 offset0 = vec3(cos(angle) * radius, sin(angle) * radius, 0.0);
    vec3 offset1 = vec3(cos(angle + 1.57079632679) * radius, sin(angle + 1.57079632679) * radius, 0.0);
    vec3 offset2 = vec3(cos(angle + 3.14159265359) * radius, sin(angle + 3.14159265359) * radius, 0.0);
    vec3 offset3 = vec3(cos(angle + 4.71238898038) * radius, sin(angle + 4.71238898038) * radius, 0.0);
    
    // Get noise at all 4 points
    float n0 = cnoise(noisePos + offset0);
    float n1 = cnoise(noisePos + offset1);
    float n2 = cnoise(noisePos + offset2);
    float n3 = cnoise(noisePos + offset3);
    
    // Smooth interpolation weights using cosine
    float w0 = (cos(angle) + 1.0) * 0.5;
    float w1 = (cos(angle + 1.57079632679) + 1.0) * 0.5;
    float w2 = (cos(angle + 3.14159265359) + 1.0) * 0.5;
    float w3 = (cos(angle + 4.71238898038) + 1.0) * 0.5;
    
    // Normalize weights
    float totalWeight = w0 + w1 + w2 + w3;
    w0 /= totalWeight;
    w1 /= totalWeight;
    w2 /= totalWeight;
    w3 /= totalWeight;
    
    // Blend all samples with amplitude boost to match single-sample strength
    // Blending reduces amplitude by ~30%, so we compensate
    float blendedNoise = n0 * w0 + n1 * w1 + n2 * w2 + n3 * w3;
    distortion = 0.75 * blendedNoise * 1.5;
  } else {
    // Normal linear time progression
    distortion = 0.75 * cnoise(noisePos + t);
  }

  vec3 pos = position + normal * distortion * uNoiseStrength * uLoadingTime;
  vPos = pos;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,Kn=`
#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
// #include <transmissionmap_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
// include를 통해 가져온 값은 대부분 환경, 빛 등을 계산하기 위해서 기본 fragment
// shader의 값들을 받아왔습니다. 일단은 무시하셔도 됩니다.

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;

varying vec3 color1;
varying vec3 color2;
varying vec3 color3;

// for npm package, need to add this manually
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {

  //-------- basic gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.5;

  #include <clipping_planes_fragment>

  vec4 diffuseColor = vec4(
      mix(mix(color1, color2, smoothstep(-3.0, 3.0, vPos.x)), color3, vPos.z),
      1);
  // diffuseColor는 오브젝트의 베이스 색상 (환경이나 빛이 고려되지 않은 본연의
  // 색)

  // mix(x, y, a): a를 축으로 했을 때 가장 낮은 값에서 x값의 영향력을 100%, 가장
  // 높은 값에서 y값의 영향력을 100%로 만든다. smoothstep(x, y, a): a축을
  // 기준으로 x를 최소값, y를 최대값으로 그 사이의 값을 쪼갠다. x와 y 사이를
  // 0-100 사이의 그라디언트처럼 단계별로 표현하고, x 미만의 값은 0, y 이상의
  // 값은 100으로 처리

  // 1. smoothstep(-3.0, 3.0,vPos.x)로 x축의 그라디언트가 표현 될 범위를 -3,
  // 3으로 정한다.
  // 2. mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x))로 color1과 color3을
  // 위의 범위 안에서 그라디언트로 표현한다.
  // 예를 들어 color1이 노랑, color3이 파랑이라고 치면, x축 기준 -3부터 3까지
  // 노랑과 파랑 사이의 그라디언트가 나타나고, -3보다 작은 값에서는 계속 노랑,
  // 3보다 큰 값에서는 계속 파랑이 나타난다.
  // 3. mix()를 한 번 더 사용해서 위의 그라디언트와 color2를 z축 기준으로
  // 분배한다.

  //-------- materiality ------------
  ReflectedLight reflectedLight =
      ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;

  #ifdef TRANSMISSION
    float totalTransmission = transmission;
  #endif
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  // #include <transmissionmap_fragment>
  #include <lights_physical_fragment>
  #include <lights_fragment_begin>
  #include <lights_fragment_maps>
  #include <lights_fragment_end>
  #include <aomap_fragment>
    vec3 outgoingLight =
        reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
        reflectedLight.directSpecular + reflectedLight.indirectSpecular;
    //위에서 정의한 diffuseColor에 환경이나 반사값들을 반영한 값.
  #ifdef TRANSMISSION
    diffuseColor.a *=
        mix(saturate(1. - totalTransmission +
                    linearToRelativeLuminance2(reflectedLight.directSpecular +
                                              reflectedLight.indirectSpecular)),
            1.0, metalness);
  #endif


  #include <tonemapping_fragment>
  #include <encodings_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>


  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  // gl_FragColor가 fragment shader를 통해 나타나는 최종값으로, diffuseColor에서
  // 정의한 그라디언트 색상 위에 반사나 빛을 계산한 값을 최종값으로 정의.
  // gl_FragColor = vec4(mix(mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x)),
  // color2, vNormal.z), 1.0); 위처럼 최종값을 그라디언트 값 자체를 넣으면 환경
  // 영향없는 그라디언트만 표현됨.
}
`,Qn=Z({fragment:()=>Kn,vertex:()=>$n}),Jn=`// #pragma glslify: pnoise = require(glsl-noise/periodic/3d)

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

// Classic Perlin noise, periodic variant
float pnoise(vec3 P, vec3 rep)
{
  vec3 Pi0 = mod(floor(P), rep); // Integer part, modulo period
  vec3 Pi1 = mod(Pi0 + vec3(1.0), rep); // Integer part + 1, mod period
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x);
  return 2.2 * n_xyz;
}


//-------- start here ------------

varying vec3 vNormal;
uniform float uTime;
uniform float uSpeed;
uniform float uLoop;
uniform float uLoopDuration;
uniform float uNoiseDensity;
uniform float uNoiseStrength;
uniform float uFrequency;
uniform float uAmplitude;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying vec3 vViewPosition;

#define STANDARD
#ifndef FLAT_SHADED
  #ifdef USE_TANGENT
    varying vec3 vTangent;
    varying vec3 vBitangent;
  #endif
#endif

#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>


// rotation
mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

void main() {
  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  float t = uTime * uSpeed;
  
  // For seamless loops, sample noise using 4D-like circular interpolation
  float distortion;
  float angle;
  
  if (uLoop > 0.5) {
    // Create truly dynamic seamless loop using 4D noise simulation
    float loopProgress = uTime / uLoopDuration;
    float loopAngle = loopProgress * 6.28318530718; // 2*PI
    
    // Radius scales with speed to maintain consistent visual speed
    float radius = 5.0 * uSpeed;
    
    // Sample 4 noise values at cardinal points
    vec3 offset0 = vec3(cos(loopAngle) * radius, sin(loopAngle) * radius, 0.0);
    vec3 offset1 = vec3(cos(loopAngle + 1.57079632679) * radius, sin(loopAngle + 1.57079632679) * radius, 0.0);
    vec3 offset2 = vec3(cos(loopAngle + 3.14159265359) * radius, sin(loopAngle + 3.14159265359) * radius, 0.0);
    vec3 offset3 = vec3(cos(loopAngle + 4.71238898038) * radius, sin(loopAngle + 4.71238898038) * radius, 0.0);
    
    // Get noise at all 4 points
    float n0 = pnoise((normal + offset0) * uNoiseDensity, vec3(10.0));
    float n1 = pnoise((normal + offset1) * uNoiseDensity, vec3(10.0));
    float n2 = pnoise((normal + offset2) * uNoiseDensity, vec3(10.0));
    float n3 = pnoise((normal + offset3) * uNoiseDensity, vec3(10.0));
    
    // Smooth interpolation weights
    float w0 = (cos(loopAngle) + 1.0) * 0.5;
    float w1 = (cos(loopAngle + 1.57079632679) + 1.0) * 0.5;
    float w2 = (cos(loopAngle + 3.14159265359) + 1.0) * 0.5;
    float w3 = (cos(loopAngle + 4.71238898038) + 1.0) * 0.5;
    
    float totalWeight = w0 + w1 + w2 + w3;
    w0 /= totalWeight;
    w1 /= totalWeight;
    w2 /= totalWeight;
    w3 /= totalWeight;
    
    // Blend samples with amplitude boost to match single-sample strength
    float blendedNoise = n0 * w0 + n1 * w1 + n2 * w2 + n3 * w3;
    distortion = blendedNoise * 1.5 * uNoiseStrength;
    
    // Apply loop to spiral effect with blended offset
    float angleOffset = offset0.x * w0 + offset1.x * w1 + offset2.x * w2 + offset3.x * w3;
    angle = sin(uv.y * uFrequency + angleOffset) * uAmplitude;
  } else {
    // Normal linear time progression
    distortion = pnoise((normal + t) * uNoiseDensity, vec3(10.0)) * uNoiseStrength;
    angle = sin(uv.y * uFrequency + t) * uAmplitude;
  }
  
  vec3 pos = position + (normal * distortion);
  pos = rotateY(pos, angle);

  vPos = pos;
  vDistort = distortion;
  vNormal = normal;
  vUv = uv;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,et=`
#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;
#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
// #include <transmissionmap_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
// include를 통해 가져온 값은 대부분 환경, 빛 등을 계산하기 위해서 기본 fragment
// shader의 값들을 받아왔습니다. 일단은 무시하셔도 됩니다.
varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;
varying vec3 color1;
varying vec3 color2;
varying vec3 color3;
varying float distanceToCenter;


// for npm package, need to add this manually
// 'linearToRelativeLuminance' : function already has a body
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {
  //-------- basic gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.5;
#include <clipping_planes_fragment>

  float distanceToCenter = distance(vPos, vec3(0, 0, 0));
  // distanceToCenter로 중심점과의 거리를 구함.

  vec4 diffuseColor =
      vec4(mix(color3, mix(color2, color1, smoothstep(-1.0, 1.0, vPos.y)),
               distanceToCenter),
           1);

  //-------- materiality ------------
  ReflectedLight reflectedLight =
      ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;
#ifdef TRANSMISSION
  float totalTransmission = transmission;
#endif
#include <logdepthbuf_fragment>
#include <map_fragment>
#include <color_fragment>
#include <alphamap_fragment>
#include <alphatest_fragment>
#include <roughnessmap_fragment>
#include <metalnessmap_fragment>
#include <normal_fragment_begin>
#include <normal_fragment_maps>
#include <clearcoat_normal_fragment_begin>
#include <clearcoat_normal_fragment_maps>
#include <emissivemap_fragment>
// #include <transmissionmap_fragment>
#include <lights_physical_fragment>
#include <lights_fragment_begin>
#include <lights_fragment_maps>
#include <lights_fragment_end>
#include <aomap_fragment>
  vec3 outgoingLight =
      reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
      reflectedLight.directSpecular + reflectedLight.indirectSpecular;
//위에서 정의한 diffuseColor에 환경이나 반사값들을 반영한 값.
#ifdef TRANSMISSION
  diffuseColor.a *=
      mix(saturate(1. - totalTransmission +
                   linearToRelativeLuminance2(reflectedLight.directSpecular +
                                             reflectedLight.indirectSpecular)),
          1.0, metalness);
#endif
  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  // gl_FragColor가 fragment shader를 통해 나타나는 최종값으로, diffuseColor에서
  // 정의한 그라디언트 색상 위에 반사나 빛을 계산한 값을 최종값으로 정의.
  // gl_FragColor = vec4(mix(mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x)),
  // color2, vNormal.z), 1.0); 위처럼 최종값을 그라디언트 값 자체를 넣으면 환경
  // 영향없는 그라디언트만 표현됨.

#include <tonemapping_fragment>
#include <encodings_fragment>
#include <fog_fragment>
#include <premultiplied_alpha_fragment>
#include <dithering_fragment>
}
`,nt=Z({fragment:()=>et,vertex:()=>Jn}),tt=`// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 
vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- start here ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);

  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

uniform float uTime;
uniform float uSpeed;
uniform float uLoop;
uniform float uLoopDuration;
uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  float t = uTime * uSpeed;
  
  // For seamless loops, sample noise using 4D-like circular interpolation
  vec3 noisePos = 0.43 * position * uNoiseDensity;
  float distortion;
  
  if (uLoop > 0.5) {
    // Create truly dynamic seamless loop using 4D noise simulation
    float loopProgress = uTime / uLoopDuration;
    float angle = loopProgress * 6.28318530718; // 2*PI
    
    // Radius scales with speed to maintain consistent visual speed
    float radius = 5.0 * uSpeed;
    
    // Sample 4 noise values at cardinal points
    vec3 offset0 = vec3(cos(angle) * radius, sin(angle) * radius, 0.0);
    vec3 offset1 = vec3(cos(angle + 1.57079632679) * radius, sin(angle + 1.57079632679) * radius, 0.0);
    vec3 offset2 = vec3(cos(angle + 3.14159265359) * radius, sin(angle + 3.14159265359) * radius, 0.0);
    vec3 offset3 = vec3(cos(angle + 4.71238898038) * radius, sin(angle + 4.71238898038) * radius, 0.0);
    
    // Get noise at all 4 points
    float n0 = cnoise(noisePos + offset0);
    float n1 = cnoise(noisePos + offset1);
    float n2 = cnoise(noisePos + offset2);
    float n3 = cnoise(noisePos + offset3);
    
    // Smooth interpolation weights
    float w0 = (cos(angle) + 1.0) * 0.5;
    float w1 = (cos(angle + 1.57079632679) + 1.0) * 0.5;
    float w2 = (cos(angle + 3.14159265359) + 1.0) * 0.5;
    float w3 = (cos(angle + 4.71238898038) + 1.0) * 0.5;
    
    float totalWeight = w0 + w1 + w2 + w3;
    w0 /= totalWeight;
    w1 /= totalWeight;
    w2 /= totalWeight;
    w3 /= totalWeight;
    
    // Blend samples with amplitude boost to match single-sample strength
    float blendedNoise = n0 * w0 + n1 * w1 + n2 * w2 + n3 * w3;
    distortion = 0.75 * blendedNoise * 1.5;
  } else {
    // Normal linear time progression
    distortion = 0.75 * cnoise(noisePos + t);
  }

  vec3 pos = position + normal * distortion * uNoiseStrength;
  vPos = pos;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,it=`
#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
// #include <transmissionmap_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
// include를 통해 가져온 값은 대부분 환경, 빛 등을 계산하기 위해서 기본 fragment
// shader의 값들을 받아왔습니다. 일단은 무시하셔도 됩니다.

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;

varying vec3 color1;
varying vec3 color2;
varying vec3 color3;

// for npm package, need to add this manually
// 'linearToRelativeLuminance' : function already has a body
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {

  //-------- basic gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.5;

  #include <clipping_planes_fragment>

  vec4 diffuseColor = vec4(
      mix(mix(color1, color2, smoothstep(-3.0, 3.0, vPos.x)), color3, vPos.z),
      1);
  // diffuseColor는 오브젝트의 베이스 색상 (환경이나 빛이 고려되지 않은 본연의
  // 색)

  // mix(x, y, a): a를 축으로 했을 때 가장 낮은 값에서 x값의 영향력을 100%, 가장
  // 높은 값에서 y값의 영향력을 100%로 만든다. smoothstep(x, y, a): a축을
  // 기준으로 x를 최소값, y를 최대값으로 그 사이의 값을 쪼갠다. x와 y 사이를
  // 0-100 사이의 그라디언트처럼 단계별로 표현하고, x 미만의 값은 0, y 이상의
  // 값은 100으로 처리

  // 1. smoothstep(-3.0, 3.0,vPos.x)로 x축의 그라디언트가 표현 될 범위를 -3,
  // 3으로 정한다.
  // 2. mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x))로 color1과 color3을
  // 위의 범위 안에서 그라디언트로 표현한다.
  // 예를 들어 color1이 노랑, color3이 파랑이라고 치면, x축 기준 -3부터 3까지
  // 노랑과 파랑 사이의 그라디언트가 나타나고, -3보다 작은 값에서는 계속 노랑,
  // 3보다 큰 값에서는 계속 파랑이 나타난다.
  // 3. mix()를 한 번 더 사용해서 위의 그라디언트와 color2를 z축 기준으로
  // 분배한다.

  //-------- materiality ------------
  ReflectedLight reflectedLight =
      ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;

  #ifdef TRANSMISSION
    float totalTransmission = transmission;
  #endif
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  // #include <transmissionmap_fragment>
  #include <lights_physical_fragment>
  #include <lights_fragment_begin>
  #include <lights_fragment_maps>
  #include <lights_fragment_end>
  #include <aomap_fragment>
    vec3 outgoingLight =
        reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
        reflectedLight.directSpecular + reflectedLight.indirectSpecular;
    //위에서 정의한 diffuseColor에 환경이나 반사값들을 반영한 값.
  #ifdef TRANSMISSION
    diffuseColor.a *=
        mix(saturate(1. - totalTransmission +
                    linearToRelativeLuminance2(reflectedLight.directSpecular +
                                              reflectedLight.indirectSpecular)),
            1.0, metalness);
  #endif


  #include <tonemapping_fragment>
  #include <encodings_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>


  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  // gl_FragColor가 fragment shader를 통해 나타나는 최종값으로, diffuseColor에서
  // 정의한 그라디언트 색상 위에 반사나 빛을 계산한 값을 최종값으로 정의.
  // gl_FragColor = vec4(mix(mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x)),
  // color2, vNormal.z), 1.0); 위처럼 최종값을 그라디언트 값 자체를 넣으면 환경
  // 영향없는 그라디언트만 표현됨.
}
`,rt=Z({fragment:()=>it,vertex:()=>tt}),ot=Z({plane:()=>Qn,sphere:()=>nt,waterPlane:()=>rt}),at=`// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- start here ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);

  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

varying vec2 vUv;

uniform float uTime;
uniform float uSpeed;

uniform float uLoadingTime;

uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  vUv = uv;

  // vNormal = normal;

  float t = uTime * uSpeed;
  // Create a sine wave from top to bottom of the sphere
  float distortion = 0.75 * cnoise(0.43 * position * uNoiseDensity + t);

  vec3 pos = position + normal * distortion * uNoiseStrength * uLoadingTime;
  vPos = pos;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,st=`uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;


varying vec3 vNormal;
varying vec3 vPos;

void main() {
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);

  gl_FragColor = vec4(color1 * vPos.x + color2 * vPos.y + color3 * vPos.z, 1.);

}
`,lt=Z({fragment:()=>st,vertex:()=>at}),ct=`// #pragma glslify: pnoise = require(glsl-noise/periodic/3d)

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

// Classic Perlin noise, periodic variant
float pnoise(vec3 P, vec3 rep)
{
  vec3 Pi0 = mod(floor(P), rep); // Integer part, modulo period
  vec3 Pi1 = mod(Pi0 + vec3(1.0), rep); // Integer part + 1, mod period
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x);
  return 2.2 * n_xyz;
}


//-------- start here ------------

varying vec3 vNormal;
uniform float uTime;
uniform float uSpeed;
uniform float uNoiseDensity;
uniform float uNoiseStrength;
uniform float uFrequency;
uniform float uAmplitude;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying vec3 vViewPosition;

#define STANDARD
#ifndef FLAT_SHADED
  #ifdef USE_TANGENT
    varying vec3 vTangent;
    varying vec3 vBitangent;
  #endif
#endif

#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>


// rotation
mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

void main() {
  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  float t = uTime * uSpeed;
  float distortion =
      pnoise((normal + t) * uNoiseDensity, vec3(10.0)) * uNoiseStrength;
  vec3 pos = position + (normal * distortion);
  float angle = sin(uv.y * uFrequency + t) * uAmplitude;
  pos = rotateY(pos, angle);

  vPos = pos;
  vDistort = distortion;
  vNormal = normal;
  vUv = uv;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,ft=`
#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif
uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;
#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
// #include <transmissionmap_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
// include를 통해 가져온 값은 대부분 환경, 빛 등을 계산하기 위해서 기본 fragment
// shader의 값들을 받아왔습니다. 일단은 무시하셔도 됩니다.
varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;
varying vec3 color1;
varying vec3 color2;
varying vec3 color3;
varying float distanceToCenter;
void main() {
  //-------- basic gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.5;
#include <clipping_planes_fragment>

  float distanceToCenter = distance(vPos, vec3(0, 0, 0));
  // distanceToCenter로 중심점과의 거리를 구함.

  vec4 diffuseColor =
      vec4(mix(color3, mix(color2, color1, smoothstep(-1.0, 1.0, vPos.y)),
               distanceToCenter),
           1);

  //-------- materiality ------------
  ReflectedLight reflectedLight =
      ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;
#ifdef TRANSMISSION
  float totalTransmission = transmission;
#endif
#include <logdepthbuf_fragment>
#include <map_fragment>
#include <color_fragment>
#include <alphamap_fragment>
#include <alphatest_fragment>
#include <roughnessmap_fragment>
#include <metalnessmap_fragment>
#include <normal_fragment_begin>
#include <normal_fragment_maps>
#include <clearcoat_normal_fragment_begin>
#include <clearcoat_normal_fragment_maps>
#include <emissivemap_fragment>
// #include <transmissionmap_fragment>
#include <lights_physical_fragment>
#include <lights_fragment_begin>
#include <lights_fragment_maps>
#include <lights_fragment_end>
#include <aomap_fragment>
  vec3 outgoingLight =
      reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
      reflectedLight.directSpecular + reflectedLight.indirectSpecular;
//위에서 정의한 diffuseColor에 환경이나 반사값들을 반영한 값.
#ifdef TRANSMISSION
  diffuseColor.a *=
      mix(saturate(1. - totalTransmission +
                   linearToRelativeLuminance(reflectedLight.directSpecular +
                                             reflectedLight.indirectSpecular)),
          1.0, metalness);
#endif
  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  // gl_FragColor가 fragment shader를 통해 나타나는 최종값으로, diffuseColor에서
  // 정의한 그라디언트 색상 위에 반사나 빛을 계산한 값을 최종값으로 정의.
  // gl_FragColor = vec4(mix(mix(color1, color3, smoothstep(-3.0, 3.0,vPos.x)),
  // color2, vNormal.z), 1.0); 위처럼 최종값을 그라디언트 값 자체를 넣으면 환경
  // 영향없는 그라디언트만 표현됨.

#include <tonemapping_fragment>
#include <encodings_fragment>
#include <fog_fragment>
#include <premultiplied_alpha_fragment>
#include <dithering_fragment>
}
`,ut=Z({fragment:()=>ft,vertex:()=>ct}),dt=`// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- start here ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);

  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;

varying vec2 vUv;

uniform float uTime;
uniform float uSpeed;

uniform float uLoadingTime;

uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- start vertex ------------
  vUv = uv;

  // vNormal = normal;

  float t = uTime * uSpeed;
  // Create a sine wave from top to bottom of the sphere
  float distortion = 0.75 * cnoise(0.43 * position * uNoiseDensity + t);

  vec3 pos = position + normal * distortion * uNoiseStrength * uLoadingTime;
  vPos = pos;

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.);
}
`,mt=`uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;


varying vec3 vNormal;
varying vec3 vPos;

void main() {
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);

  gl_FragColor = vec4(color1 * vPos.x + color2 * vPos.y + color3 * vPos.z, 1.);

}
`,gt=Z({fragment:()=>mt,vertex:()=>dt}),vt=Z({plane:()=>lt,sphere:()=>ut,waterPlane:()=>gt}),pt=`// Cosmic Plane Vertex Shader - Holographic Effect
// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- Holographic Effect Functions ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

mat3 rotation3dX(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(1.0, 0.0, 0.0, 0.0, c, s, 0.0, -s, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }
vec3 rotateX(vec3 v, float angle) { return rotation3dX(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vHolographicIntensity;
varying float vCosmicWave;

uniform float uTime;
uniform float uSpeed;
uniform float uLoadingTime;
uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- Cosmic Holographic Effect ------------
  vUv = uv;
  
  float t = uTime * uSpeed;
  
  // Create holographic interference patterns
  float holographicPattern = sin(position.x * 15.0 + t * 2.0) * 
                            sin(position.y * 12.0 + t * 1.5) * 0.1;
  
  // Cosmic wave distortion
  float cosmicWave = cnoise(position * uNoiseDensity * 0.5 + vec3(t * 0.3, t * 0.2, t * 0.4));
  vCosmicWave = cosmicWave;
  
  // Multi-layer noise for depth
  float noise1 = cnoise(position * uNoiseDensity * 2.0 + t * 0.8);
  float noise2 = cnoise(position * uNoiseDensity * 0.3 + t * 0.2) * 0.5;
  float noise3 = cnoise(position * uNoiseDensity * 4.0 + t * 1.2) * 0.25;
  
  float combinedNoise = noise1 + noise2 + noise3;
  
  // Holographic shimmer effect
  float shimmer = sin(position.x * 30.0 + t * 4.0) * 
                  cos(position.y * 25.0 + t * 3.0) * 0.05;
  
  // Calculate holographic intensity for fragment shader
  vHolographicIntensity = abs(holographicPattern) + abs(shimmer) * 2.0;
  
  // Apply displacement with holographic and cosmic effects
  float totalDisplacement = (combinedNoise + holographicPattern + shimmer) * uNoiseStrength * uLoadingTime;
  
  vec3 pos = position + normal * totalDisplacement;
  vPos = pos;
  
  // Add subtle rotation effect for cosmic feel
  pos = rotateY(pos, sin(t * 0.1) * 0.05);
  pos = rotateX(pos, cos(t * 0.07) * 0.03);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.0);
}
`,ht=`// Cosmic Plane Fragment Shader - Holographic Gradient

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vHolographicIntensity;
varying float vCosmicWave;

uniform float uTime;
uniform float uSpeed;

uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;

// Holographic helper functions
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float noise2D(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    
    return mix(mix(hash(i + vec2(0.0, 0.0)), 
                   hash(i + vec2(1.0, 0.0)), u.x),
               mix(hash(i + vec2(0.0, 1.0)), 
                   hash(i + vec2(1.0, 1.0)), u.x), u.y);
}

// for npm package, need to add this manually
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {

  //-------- Cosmic Holographic Gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.2; // More reflective for holographic effect

  #include <clipping_planes_fragment>

  float t = uTime * uSpeed;
  
  // Create holographic interference patterns
  float interference1 = sin(vPos.x * 20.0 + t * 3.0) * cos(vPos.y * 15.0 + t * 2.0);
  float interference2 = sin(vPos.x * 35.0 + t * 4.0) * sin(vPos.y * 30.0 + t * 3.5);
  float interference3 = cos(vPos.x * 50.0 + t * 5.0) * cos(vPos.y * 45.0 + t * 4.5);
  
  // Combine interference patterns
  float holographicPattern = (interference1 + interference2 * 0.5 + interference3 * 0.25) / 1.75;
  
  // Create cosmic shimmer effect
  float shimmer = noise2D(vPos.xy * 40.0 + t * 2.0) * 0.3;
  float cosmicGlow = noise2D(vPos.xy * 8.0 + t * 0.5) * 0.5;
  
  // Holographic color shifting
  vec3 holographicShift = vec3(
    sin(vPos.x * 10.0 + t * 2.0 + 0.0) * 0.1,
    sin(vPos.x * 10.0 + t * 2.0 + 2.094) * 0.1,  // 120 degrees
    sin(vPos.x * 10.0 + t * 2.0 + 4.188) * 0.1   // 240 degrees
  );
  
  // Enhanced gradient mixing with cosmic effects
  float gradientX = smoothstep(-4.0, 4.0, vPos.x + holographicPattern * 2.0);
  float gradientY = smoothstep(-4.0, 4.0, vPos.y + vCosmicWave * 1.5);
  float gradientZ = smoothstep(-2.0, 2.0, vPos.z + shimmer);
  
  // Multi-layer color mixing for depth
  vec3 baseGradient = mix(
    mix(color1, color2, gradientX), 
    color3, 
    gradientY * 0.7 + gradientZ * 0.3
  );
  
  // Apply holographic color shifts
  vec3 holographicColor = baseGradient + holographicShift;
  
  // Add cosmic glow and shimmer
  vec3 cosmicEnhancement = vec3(
    cosmicGlow * 0.2,
    shimmer * 0.15,
    (cosmicGlow + shimmer) * 0.1
  );
  
  // Holographic intensity modulation
  float intensityMod = 1.0 + vHolographicIntensity * 0.5 + abs(holographicPattern) * 0.3;
  
  // Final color with cosmic and holographic effects
  vec3 finalColor = (holographicColor + cosmicEnhancement) * intensityMod;
  
  // Add subtle iridescence
  float iridescence = sin(vPos.x * 25.0 + t * 3.0) * cos(vPos.y * 20.0 + t * 2.5) * 0.1;
  finalColor += vec3(iridescence * 0.2, iridescence * 0.3, iridescence * 0.4);

  vec4 diffuseColor = vec4(finalColor, 1.0);

  //-------- Enhanced Materiality for Holographic Effect ------------
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive + finalColor * 0.1; // Add some emission for glow

  #ifdef TRANSMISSION
    float totalTransmission = transmission;
  #endif
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  #include <lights_physical_fragment>
  #include <lights_fragment_begin>
  #include <lights_fragment_maps>
  #include <lights_fragment_end>
  #include <aomap_fragment>
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
                      reflectedLight.directSpecular + reflectedLight.indirectSpecular +
                      totalEmissiveRadiance;

  #ifdef TRANSMISSION
    diffuseColor.a *= mix(saturate(1. - totalTransmission +
                        linearToRelativeLuminance2(reflectedLight.directSpecular +
                                                  reflectedLight.indirectSpecular)),
                1.0, metalness);
  #endif

  #include <tonemapping_fragment>
  #include <encodings_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>

  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
}
`,_t=Z({fragment:()=>ht,vertex:()=>pt}),yt=`// Cosmic Sphere Vertex Shader - Nebula Effect
// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- Nebula Effect Functions ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

mat3 rotation3dX(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(1.0, 0.0, 0.0, 0.0, c, s, 0.0, -s, c);
}

mat3 rotation3dZ(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, s, 0.0, -s, c, 0.0, 0.0, 0.0, 1.0);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }
vec3 rotateX(vec3 v, float angle) { return rotation3dX(angle) * v; }
vec3 rotateZ(vec3 v, float angle) { return rotation3dZ(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vNebulaIntensity;
varying float vParticleDensity;
varying vec3 vCosmicSwirl;

uniform float uTime;
uniform float uSpeed;
uniform float uLoadingTime;
uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- Cosmic Nebula Effect ------------
  vUv = uv;
  
  float t = uTime * uSpeed;
  
  // Create swirling nebula patterns
  vec3 swirlCenter = vec3(0.0, 0.0, 0.0);
  vec3 toCenter = position - swirlCenter;
  float distanceFromCenter = length(toCenter);
  
  // Create spiral motion
  float angle = atan(toCenter.y, toCenter.x);
  float spiralAngle = angle + distanceFromCenter * 2.0 + t * 0.5;
  
  // Multi-octave noise for nebula density
  float nebula1 = cnoise(position * uNoiseDensity * 0.8 + vec3(t * 0.2, t * 0.3, t * 0.1));
  float nebula2 = cnoise(position * uNoiseDensity * 1.5 + vec3(t * 0.4, t * 0.2, t * 0.5)) * 0.7;
  float nebula3 = cnoise(position * uNoiseDensity * 3.0 + vec3(t * 0.8, t * 0.6, t * 0.9)) * 0.4;
  float nebula4 = cnoise(position * uNoiseDensity * 6.0 + vec3(t * 1.2, t * 1.0, t * 1.4)) * 0.2;
  
  // Combine nebula layers for complexity
  float nebulaPattern = nebula1 + nebula2 + nebula3 + nebula4;
  vNebulaIntensity = abs(nebulaPattern);
  
  // Create particle-like density variations
  float particleDensity = cnoise(position * uNoiseDensity * 8.0 + vec3(t * 2.0, t * 1.5, t * 2.5));
  vParticleDensity = smoothstep(-0.3, 0.8, particleDensity);
  
  // Create cosmic swirl effect
  vec3 swirl = vec3(
    sin(spiralAngle + t * 0.3) * distanceFromCenter * 0.1,
    cos(spiralAngle + t * 0.2) * distanceFromCenter * 0.1,
    sin(distanceFromCenter * 3.0 + t * 0.4) * 0.05
  );
  vCosmicSwirl = swirl;
  
  // Create pulsing effect for cosmic energy
  float pulse = sin(t * 2.0 + distanceFromCenter * 5.0) * 0.1 + 1.0;
  
  // Apply complex displacement
  float totalDisplacement = nebulaPattern * uNoiseStrength * uLoadingTime * pulse;
  
  // Add swirl displacement
  vec3 pos = position + normal * totalDisplacement + swirl * 0.3;
  vPos = pos;
  
  // Add cosmic rotation for dynamic feel
  pos = rotateY(pos, sin(t * 0.1 + distanceFromCenter) * 0.1);
  pos = rotateX(pos, cos(t * 0.08 + angle) * 0.08);
  pos = rotateZ(pos, sin(t * 0.05 + spiralAngle) * 0.05);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.0);
}
`,xt=`// Cosmic Sphere Fragment Shader - Nebula Particle Effect

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vNebulaIntensity;
varying float vParticleDensity;
varying vec3 vCosmicSwirl;

uniform float uTime;
uniform float uSpeed;

uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;

// Nebula helper functions
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float noise2D(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    
    return mix(mix(hash(i + vec2(0.0, 0.0)), 
                   hash(i + vec2(1.0, 0.0)), u.x),
               mix(hash(i + vec2(0.0, 1.0)), 
                   hash(i + vec2(1.0, 1.0)), u.x), u.y);
}

// Fractal Brownian Motion for complex nebula patterns
float fbm(vec2 p) {
    float value = 0.0;
    float amplitude = 0.5;
    float frequency = 1.0;
    
    for(int i = 0; i < 5; i++) {
        value += amplitude * noise2D(p * frequency);
        amplitude *= 0.5;
        frequency *= 2.0;
    }
    return value;
}

// Star field generation
float stars(vec2 p, float density) {
    vec2 n = floor(p * density);
    vec2 f = fract(p * density);
    
    float d = 1.0;
    for(int i = -1; i <= 1; i++) {
        for(int j = -1; j <= 1; j++) {
            vec2 g = vec2(float(i), float(j));
            vec2 o = hash(n + g) * vec2(1.0);
            vec2 r = g + o - f;
            d = min(d, dot(r, r));
        }
    }
    
    return 1.0 - smoothstep(0.0, 0.02, sqrt(d));
}

// for npm package, need to add this manually
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {

  //-------- Cosmic Nebula Gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.1; // Very reflective for cosmic shine

  #include <clipping_planes_fragment>

  float t = uTime * uSpeed;
  
  // Calculate distance from center for radial effects
  float distanceFromCenter = length(vPos);
  float angle = atan(vPos.y, vPos.x);
  
  // Create complex nebula patterns using FBM
  vec2 nebulaCoords = vPos.xy * 3.0 + vCosmicSwirl.xy;
  float nebulaPattern1 = fbm(nebulaCoords + t * 0.1);
  float nebulaPattern2 = fbm(nebulaCoords * 2.0 + t * 0.15);
  float nebulaPattern3 = fbm(nebulaCoords * 4.0 + t * 0.2);
  
  // Combine nebula patterns
  float combinedNebula = (nebulaPattern1 + nebulaPattern2 * 0.5 + nebulaPattern3 * 0.25) / 1.75;
  
  // Create particle-like bright spots
  float particleField = stars(vPos.xy * 20.0 + t * 0.5, 50.0);
  float microParticles = stars(vPos.xy * 80.0 + t * 1.0, 200.0) * 0.5;
  
  // Create cosmic dust clouds
  float dustClouds = fbm(vPos.xy * 8.0 + t * 0.05) * 0.3;
  
  // Energy streams
  float energyStream1 = sin(vPos.x * 15.0 + t * 3.0 + angle * 2.0) * 0.1;
  float energyStream2 = cos(vPos.y * 20.0 + t * 2.5 + distanceFromCenter * 5.0) * 0.1;
  
  // Cosmic gradient mixing with nebula influence
  float gradientX = smoothstep(-3.0, 3.0, vPos.x + combinedNebula * 2.0 + vCosmicSwirl.x * 3.0);
  float gradientY = smoothstep(-3.0, 3.0, vPos.y + vNebulaIntensity * 1.5 + vCosmicSwirl.y * 2.0);
  float gradientZ = smoothstep(-2.0, 2.0, vPos.z + dustClouds * 2.0);
  
  // Multi-layer color mixing
  vec3 baseGradient = mix(
    mix(color1, color2, gradientX), 
    color3, 
    gradientY * 0.6 + gradientZ * 0.4
  );
  
  // Add nebula color variations
  vec3 nebulaColor = baseGradient;
  nebulaColor.r += combinedNebula * 0.3 + energyStream1;
  nebulaColor.g += vNebulaIntensity * 0.2 + energyStream2;
  nebulaColor.b += dustClouds * 0.4 + abs(vCosmicSwirl.z) * 0.5;
  
  // Add particle brightness
  vec3 particleGlow = vec3(
    particleField * 0.8 + microParticles * 0.4,
    particleField * 0.6 + microParticles * 0.3,
    particleField * 0.9 + microParticles * 0.5
  );
  
  // Create pulsing cosmic energy
  float cosmicPulse = sin(t * 1.5 + distanceFromCenter * 3.0) * 0.1 + 1.0;
  
  // Combine all effects
  vec3 finalColor = (nebulaColor + particleGlow * 2.0) * cosmicPulse;
  
  // Add cosmic rim lighting effect
  float rimLight = pow(1.0 - abs(dot(normalize(vNormal), normalize(vViewPosition))), 2.0);
  finalColor += rimLight * 0.3 * (color1 + color2 + color3) / 3.0;
  
  // Enhance particle density areas
  finalColor = mix(finalColor, finalColor * 1.5, vParticleDensity * 0.5);
  
  // Add subtle color temperature variation
  float temperature = sin(angle * 3.0 + t * 0.8) * 0.1;
  finalColor.r += temperature * 0.1;
  finalColor.b -= temperature * 0.1;

  vec4 diffuseColor = vec4(finalColor, 1.0);

  //-------- Enhanced Materiality for Cosmic Effect ------------
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive + finalColor * 0.2; // Strong emission for nebula glow

  #ifdef TRANSMISSION
    float totalTransmission = transmission;
  #endif
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  #include <lights_physical_fragment>
  #include <lights_fragment_begin>
  #include <lights_fragment_maps>
  #include <lights_fragment_end>
  #include <aomap_fragment>
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
                      reflectedLight.directSpecular + reflectedLight.indirectSpecular +
                      totalEmissiveRadiance;

  #ifdef TRANSMISSION
    diffuseColor.a *= mix(saturate(1. - totalTransmission +
                        linearToRelativeLuminance2(reflectedLight.directSpecular +
                                                  reflectedLight.indirectSpecular)),
                1.0, metalness);
  #endif

  #include <tonemapping_fragment>
  #include <encodings_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>

  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
}
`,Ct=Z({fragment:()=>xt,vertex:()=>yt}),Pt=`// Cosmic WaterPlane Vertex Shader - Aurora Wave Effect
// #pragma glslify: cnoise3 = require(glsl-noise/classic/3d) 

// noise source from https://github.com/hughsk/glsl-noise/blob/master/periodic/3d.glsl

vec3 mod289(vec3 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x)
{
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x)
{
  return mod289(((x*34.0)+1.0)*x);
}

vec4 taylorInvSqrt(vec4 r)
{
  return 1.79284291400159 - 0.85373472095314 * r;
}

vec3 fade(vec3 t) {
  return t*t*t*(t*(t*6.0-15.0)+10.0);
}

float cnoise(vec3 P)
{
  vec3 Pi0 = floor(P); // Integer part for indexing
  vec3 Pi1 = Pi0 + vec3(1.0); // Integer part + 1
  Pi0 = mod289(Pi0);
  Pi1 = mod289(Pi1);
  vec3 Pf0 = fract(P); // Fractional part for interpolation
  vec3 Pf1 = Pf0 - vec3(1.0); // Fractional part - 1.0
  vec4 ix = vec4(Pi0.x, Pi1.x, Pi0.x, Pi1.x);
  vec4 iy = vec4(Pi0.yy, Pi1.yy);
  vec4 iz0 = Pi0.zzzz;
  vec4 iz1 = Pi1.zzzz;

  vec4 ixy = permute(permute(ix) + iy);
  vec4 ixy0 = permute(ixy + iz0);
  vec4 ixy1 = permute(ixy + iz1);

  vec4 gx0 = ixy0 * (1.0 / 7.0);
  vec4 gy0 = fract(floor(gx0) * (1.0 / 7.0)) - 0.5;
  gx0 = fract(gx0);
  vec4 gz0 = vec4(0.5) - abs(gx0) - abs(gy0);
  vec4 sz0 = step(gz0, vec4(0.0));
  gx0 -= sz0 * (step(0.0, gx0) - 0.5);
  gy0 -= sz0 * (step(0.0, gy0) - 0.5);

  vec4 gx1 = ixy1 * (1.0 / 7.0);
  vec4 gy1 = fract(floor(gx1) * (1.0 / 7.0)) - 0.5;
  gx1 = fract(gx1);
  vec4 gz1 = vec4(0.5) - abs(gx1) - abs(gy1);
  vec4 sz1 = step(gz1, vec4(0.0));
  gx1 -= sz1 * (step(0.0, gx1) - 0.5);
  gy1 -= sz1 * (step(0.0, gy1) - 0.5);

  vec3 g000 = vec3(gx0.x,gy0.x,gz0.x);
  vec3 g100 = vec3(gx0.y,gy0.y,gz0.y);
  vec3 g010 = vec3(gx0.z,gy0.z,gz0.z);
  vec3 g110 = vec3(gx0.w,gy0.w,gz0.w);
  vec3 g001 = vec3(gx1.x,gy1.x,gz1.x);
  vec3 g101 = vec3(gx1.y,gy1.y,gz1.y);
  vec3 g011 = vec3(gx1.z,gy1.z,gz1.z);
  vec3 g111 = vec3(gx1.w,gy1.w,gz1.w);

  vec4 norm0 = taylorInvSqrt(vec4(dot(g000, g000), dot(g010, g010), dot(g100, g100), dot(g110, g110)));
  g000 *= norm0.x;
  g010 *= norm0.y;
  g100 *= norm0.z;
  g110 *= norm0.w;
  vec4 norm1 = taylorInvSqrt(vec4(dot(g001, g001), dot(g011, g011), dot(g101, g101), dot(g111, g111)));
  g001 *= norm1.x;
  g011 *= norm1.y;
  g101 *= norm1.z;
  g111 *= norm1.w;

  float n000 = dot(g000, Pf0);
  float n100 = dot(g100, vec3(Pf1.x, Pf0.yz));
  float n010 = dot(g010, vec3(Pf0.x, Pf1.y, Pf0.z));
  float n110 = dot(g110, vec3(Pf1.xy, Pf0.z));
  float n001 = dot(g001, vec3(Pf0.xy, Pf1.z));
  float n101 = dot(g101, vec3(Pf1.x, Pf0.y, Pf1.z));
  float n011 = dot(g011, vec3(Pf0.x, Pf1.yz));
  float n111 = dot(g111, Pf1);

  vec3 fade_xyz = fade(Pf0);
  vec4 n_z = mix(vec4(n000, n100, n010, n110), vec4(n001, n101, n011, n111), fade_xyz.z);
  vec2 n_yz = mix(n_z.xy, n_z.zw, fade_xyz.y);
  float n_xyz = mix(n_yz.x, n_yz.y, fade_xyz.x); 
  return 2.2 * n_xyz;
}

//-------- Aurora Wave Effect Functions ------------

mat3 rotation3dY(float angle) {
  float s = sin(angle);
  float c = cos(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

vec3 rotateY(vec3 v, float angle) { return rotation3dY(angle) * v; }

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vAuroraIntensity;
varying float vWaveHeight;
varying vec3 vFlowDirection;

uniform float uTime;
uniform float uSpeed;
uniform float uLoadingTime;
uniform float uNoiseDensity;
uniform float uNoiseStrength;

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

void main() {

  #include <beginnormal_vertex>
  #include <color_vertex>
  #include <defaultnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <uv2_vertex>
  #include <uv_vertex>
  #ifndef FLAT_SHADED
    vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
    vTangent = normalize(transformedTangent);
    vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  #include <begin_vertex>

  #include <clipping_planes_vertex>
  #include <displacementmap_vertex>
  #include <logdepthbuf_vertex>
  #include <morphtarget_vertex>
  #include <project_vertex>
  #include <skinning_vertex>
    vViewPosition = -mvPosition.xyz;
  #include <fog_vertex>
  #include <shadowmap_vertex>
  #include <worldpos_vertex>

  //-------- Cosmic Aurora Wave Effect ------------
  vUv = uv;
  
  float t = uTime * uSpeed;
  
  // Create flowing aurora patterns
  float auroraFlow1 = sin(position.x * 5.0 + t * 1.5) * cos(position.y * 3.0 + t * 1.0);
  float auroraFlow2 = sin(position.x * 8.0 + t * 2.0) * sin(position.y * 6.0 + t * 1.8);
  float auroraFlow3 = cos(position.x * 12.0 + t * 2.5) * cos(position.y * 9.0 + t * 2.2);
  
  // Combine aurora flows
  float auroraPattern = (auroraFlow1 + auroraFlow2 * 0.7 + auroraFlow3 * 0.4) / 2.1;
  vAuroraIntensity = abs(auroraPattern);
  
  // Create multi-layered waves
  float wave1 = cnoise(vec3(position.xy * uNoiseDensity * 0.5, t * 0.3));
  float wave2 = cnoise(vec3(position.xy * uNoiseDensity * 1.2, t * 0.5)) * 0.6;
  float wave3 = cnoise(vec3(position.xy * uNoiseDensity * 2.5, t * 0.8)) * 0.3;
  float wave4 = cnoise(vec3(position.xy * uNoiseDensity * 5.0, t * 1.2)) * 0.15;
  
  // Combine waves for complex water surface
  float combinedWaves = wave1 + wave2 + wave3 + wave4;
  vWaveHeight = combinedWaves;
  
  // Create flowing current patterns
  vec2 flowDirection = vec2(
    sin(position.x * 2.0 + t * 0.8) + cos(position.y * 1.5 + t * 0.6),
    cos(position.x * 1.8 + t * 0.7) + sin(position.y * 2.2 + t * 0.9)
  );
  vFlowDirection = vec3(normalize(flowDirection), 0.0);
  
  // Aurora-influenced wave distortion
  float auroraWave = sin(position.x * 15.0 + t * 3.0 + auroraPattern * 5.0) * 
                     cos(position.y * 12.0 + t * 2.5 + auroraPattern * 4.0) * 0.2;
  
  // Create cosmic energy ripples
  float distanceFromCenter = length(position.xy);
  float cosmicRipple = sin(distanceFromCenter * 8.0 - t * 4.0) * 
                       exp(-distanceFromCenter * 0.1) * 0.3;
  
  // Pulsing effect for cosmic energy
  float cosmicPulse = sin(t * 1.5 + distanceFromCenter * 2.0) * 0.1 + 1.0;
  
  // Apply complex displacement
  float totalDisplacement = (combinedWaves + auroraWave + cosmicRipple) * 
                           uNoiseStrength * uLoadingTime * cosmicPulse;
  
  vec3 pos = position + normal * totalDisplacement;
  vPos = pos;
  
  // Add subtle rotation for cosmic flow
  pos = rotateY(pos, sin(t * 0.05 + distanceFromCenter * 0.1) * 0.02);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.0);
}
`,Tt=`// Cosmic WaterPlane Fragment Shader - Aurora Wave Effect

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

#ifdef TRANSMISSION
uniform float transmission;
#endif
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <cube_uv_reflection_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>

varying vec3 vNormal;
varying float displacement;
varying vec3 vPos;
varying float vDistort;
varying vec2 vUv;
varying float vAuroraIntensity;
varying float vWaveHeight;
varying vec3 vFlowDirection;

uniform float uTime;
uniform float uSpeed;

uniform float uC1r;
uniform float uC1g;
uniform float uC1b;
uniform float uC2r;
uniform float uC2g;
uniform float uC2b;
uniform float uC3r;
uniform float uC3g;
uniform float uC3b;

// Aurora helper functions
float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float noise2D(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    
    return mix(mix(hash(i + vec2(0.0, 0.0)), 
                   hash(i + vec2(1.0, 0.0)), u.x),
               mix(hash(i + vec2(0.0, 1.0)), 
                   hash(i + vec2(1.0, 1.0)), u.x), u.y);
}

// Fractal Brownian Motion for aurora patterns
float fbm(vec2 p) {
    float value = 0.0;
    float amplitude = 0.5;
    float frequency = 1.0;
    
    for(int i = 0; i < 4; i++) {
        value += amplitude * noise2D(p * frequency);
        amplitude *= 0.5;
        frequency *= 2.0;
    }
    return value;
}

// Aurora curtain effect
float aurora(vec2 p, float time) {
    vec2 q = vec2(fbm(p + vec2(0.0, time * 0.1)),
                  fbm(p + vec2(5.2, time * 0.15)));
    
    vec2 r = vec2(fbm(p + 4.0 * q + vec2(1.7, time * 0.2)),
                  fbm(p + 4.0 * q + vec2(8.3, time * 0.18)));
    
    return fbm(p + 4.0 * r);
}

// Water caustics effect
float caustics(vec2 p, float time) {
    vec2 uv = p * 4.0;
    vec2 p0 = uv + vec2(time * 0.3, time * 0.2);
    vec2 p1 = uv + vec2(time * -0.4, time * 0.3);
    
    float c1 = sin(length(p0) * 8.0 - time * 2.0) * 0.5 + 0.5;
    float c2 = sin(length(p1) * 6.0 - time * 1.5) * 0.5 + 0.5;
    
    return (c1 + c2) * 0.5;
}

// for npm package, need to add this manually
float linearToRelativeLuminance2( const in vec3 color ) {
    vec3 weights = vec3( 0.2126, 0.7152, 0.0722 );
    return dot( weights, color.rgb );
}

void main() {

  //-------- Cosmic Aurora Water Gradient ------------
  vec3 color1 = vec3(uC1r, uC1g, uC1b);
  vec3 color2 = vec3(uC2r, uC2g, uC2b);
  vec3 color3 = vec3(uC3r, uC3g, uC3b);
  
  float clearcoat = 1.0;
  float clearcoatRoughness = 0.05; // Very smooth for water-like reflection

  #include <clipping_planes_fragment>

  float t = uTime * uSpeed;
  
  // Create aurora patterns
  vec2 auroraCoords = vPos.xy * 2.0 + vFlowDirection.xy * t * 0.5;
  float auroraPattern1 = aurora(auroraCoords, t);
  float auroraPattern2 = aurora(auroraCoords * 1.5 + vec2(3.0, 1.0), t * 1.2);
  float auroraPattern3 = aurora(auroraCoords * 0.7 + vec2(-2.0, 4.0), t * 0.8);
  
  // Combine aurora layers
  float combinedAurora = (auroraPattern1 + auroraPattern2 * 0.7 + auroraPattern3 * 0.5) / 2.2;
  
  // Create water caustics
  float causticsPattern = caustics(vPos.xy, t);
  
  // Create flowing light streams
  float lightStream1 = sin(vPos.x * 8.0 + t * 2.0 + combinedAurora * 3.0) * 0.2;
  float lightStream2 = cos(vPos.y * 6.0 + t * 1.5 + vWaveHeight * 4.0) * 0.15;
  float lightStream3 = sin((vPos.x + vPos.y) * 10.0 + t * 2.5) * 0.1;
  
  // Create cosmic energy waves
  float distanceFromCenter = length(vPos.xy);
  float energyWave = sin(distanceFromCenter * 5.0 - t * 3.0) * 
                     exp(-distanceFromCenter * 0.05) * 0.3;
  
  // Aurora color shifting effect
  vec3 auroraShift = vec3(
    sin(combinedAurora * 6.28 + t * 1.0) * 0.2,
    sin(combinedAurora * 6.28 + t * 1.0 + 2.094) * 0.2,  // 120 degrees
    sin(combinedAurora * 6.28 + t * 1.0 + 4.188) * 0.2   // 240 degrees
  );
  
  // Enhanced gradient mixing with aurora and water effects
  float gradientX = smoothstep(-4.0, 4.0, vPos.x + combinedAurora * 3.0 + vFlowDirection.x * 2.0);
  float gradientY = smoothstep(-4.0, 4.0, vPos.y + vWaveHeight * 2.0 + lightStream1 * 3.0);
  float gradientZ = smoothstep(-3.0, 3.0, vPos.z + causticsPattern * 2.0);
  
  // Multi-layer color mixing
  vec3 baseGradient = mix(
    mix(color1, color2, gradientX), 
    color3, 
    gradientY * 0.7 + gradientZ * 0.3
  );
  
  // Apply aurora color shifts
  vec3 auroraColor = baseGradient + auroraShift;
  
  // Add water caustics coloring
  vec3 causticsColor = vec3(
    causticsPattern * 0.3,
    causticsPattern * 0.4,
    causticsPattern * 0.5
  );
  
  // Add light streams
  vec3 lightStreams = vec3(
    abs(lightStream1) * 0.4,
    abs(lightStream2) * 0.3,
    abs(lightStream3) * 0.5
  );
  
  // Aurora intensity modulation
  float auroraIntensityMod = 1.0 + vAuroraIntensity * 0.8 + abs(combinedAurora) * 0.6;
  
  // Combine all effects
  vec3 finalColor = (auroraColor + causticsColor + lightStreams + vec3(energyWave * 0.2)) * auroraIntensityMod;
  
  // Add water-like shimmer
  float shimmer = sin(vPos.x * 20.0 + t * 4.0) * 
                  cos(vPos.y * 18.0 + t * 3.5) * 
                  vWaveHeight * 0.1;
  finalColor += vec3(shimmer * 0.3, shimmer * 0.4, shimmer * 0.6);
  
  // Add aurora dancing effect
  float auroraMovement = sin(vPos.x * 3.0 + t * 1.2 + combinedAurora * 2.0) * 
                         cos(vPos.y * 2.5 + t * 0.9) * 0.15;
  finalColor.g += abs(auroraMovement) * 0.4;
  finalColor.b += abs(auroraMovement) * 0.2;
  
  // Add cosmic depth variation
  float depthVariation = noise2D(vPos.xy * 5.0 + t * 0.3) * 0.1;
  finalColor *= (1.0 + depthVariation);

  vec4 diffuseColor = vec4(finalColor, 1.0);

  //-------- Enhanced Materiality for Water Aurora Effect ------------
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive + finalColor * 0.15; // Moderate emission for aurora glow

  #ifdef TRANSMISSION
    float totalTransmission = transmission;
  #endif
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  #include <lights_physical_fragment>
  #include <lights_fragment_begin>
  #include <lights_fragment_maps>
  #include <lights_fragment_end>
  #include <aomap_fragment>
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse +
                      reflectedLight.directSpecular + reflectedLight.indirectSpecular +
                      totalEmissiveRadiance;

  #ifdef TRANSMISSION
    diffuseColor.a *= mix(saturate(1. - totalTransmission +
                        linearToRelativeLuminance2(reflectedLight.directSpecular +
                                                  reflectedLight.indirectSpecular)),
                1.0, metalness);
  #endif

  #include <tonemapping_fragment>
  #include <encodings_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>

  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
}
`,zt=Z({fragment:()=>Tt,vertex:()=>Pt}),wt=Z({plane:()=>_t,sphere:()=>Ct,waterPlane:()=>zt}),Et=`// Glass Plane Vertex Shader - Refraction & Transparency Effects

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;

uniform float uTime;
uniform float uSpeed;
uniform float uWaveAmplitude;
uniform float uWaveFrequency;
uniform float uNoiseStrength;
uniform float uDistortion;

// Noise functions for glass distortion
vec3 mod289(vec3 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x) {
  return mod289(((x * 34.0) + 1.0) * x);
}

vec4 taylorInvSqrt(vec4 r) {
  return 1.79284291400159 - 0.85373472095314 * r;
}

float snoise(vec3 v) {
  const vec2 C = vec2(1.0 / 6.0, 1.0 / 3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);

  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);

  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);

  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;

  i = mod289(i);
  vec4 p = permute(permute(permute(
    i.z + vec4(0.0, i1.z, i2.z, 1.0))
    + i.y + vec4(0.0, i1.y, i2.y, 1.0))
    + i.x + vec4(0.0, i1.x, i2.x, 1.0));

  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;

  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);

  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);

  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);

  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);

  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));

  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;

  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);

  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x;
  p1 *= norm.y;
  p2 *= norm.z;
  p3 *= norm.w;

  vec4 m = max(0.6 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1),
    dot(p2, x2), dot(p3, x3)));
}

void main() {
  #include <uv_pars_vertex>
  #include <uv_vertex>
  #include <uv2_pars_vertex>
  #include <uv2_vertex>
  #include <color_pars_vertex>
  #include <color_vertex>
  #include <morphcolor_vertex>
  #include <beginnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <defaultnormal_vertex>
  #include <normal_vertex>
  
  #ifndef FLAT_SHADED
  vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
  vTangent = normalize(transformedTangent);
  vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  
  #include <begin_vertex>
  #include <morphtarget_vertex>
  #include <skinning_vertex>
  #include <displacementmap_vertex>
  
  // Pass UV coordinates
  vUv = uv;

  // Calculate time-based animation
  float time = uTime * uSpeed;
  
  // Create subtle wave distortion for glass effect
  float waveX = sin(position.x * uWaveFrequency + time) * uWaveAmplitude;
  float waveY = cos(position.y * uWaveFrequency + time) * uWaveAmplitude;
  float waveZ = sin(position.z * uWaveFrequency + time * 0.5) * uWaveAmplitude * 0.5;
  
  // Add noise for organic glass distortion
  vec3 noisePos = position + vec3(time * 0.1);
  float noise = snoise(noisePos * 0.5) * uNoiseStrength;
  
  // Apply distortion to transformed position
  transformed += vec3(waveX, waveY, waveZ) * uDistortion + normal * noise;
  
  #include <project_vertex>
  #include <logdepthbuf_vertex>
  #include <clipping_planes_vertex>
  
  vViewPosition = -mvPosition.xyz;
  vPosition = transformed;
  
  // Calculate world position for refraction
  vec4 worldPosition = modelMatrix * vec4(transformed, 1.0);
  vGlassWorldPos = worldPosition.xyz;
  
  // Calculate reflection and refraction vectors
  vec3 worldNormal = normalize(mat3(modelMatrix) * normal);
  vec3 viewVector = normalize(cameraPosition - worldPosition.xyz);
  
  // Reflection vector
  vReflect = reflect(-viewVector, worldNormal);
  
  // Refraction vector with index of refraction for glass (1.5)
  float ior = 1.5;
  vRefract = refract(-viewVector, worldNormal, 1.0 / ior);
  
  #include <fog_vertex>
  #include <shadowmap_vertex>
}
`,bt=`// Glass Plane Fragment Shader - Transparency & Refraction

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

// transmission is already defined by Three.js when TRANSMISSION is enabled
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif

varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif

#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
#include <transmission_pars_fragment>

// Custom uniforms for glass effect
uniform float uTime;
uniform vec3 uColor1;
uniform vec3 uColor2;
uniform vec3 uColor3;
uniform float uTransparency;
uniform float uRefraction;
uniform float uChromaticAberration;
uniform float uFresnelPower;
uniform float uReflectivity;
// envMap and envMapIntensity are provided by Three.js

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;

// Fresnel calculation
float fresnel(vec3 viewDirection, vec3 normal, float power) {
  return pow(1.0 - dot(viewDirection, normal), power);
}

// Chromatic aberration for refraction
vec3 chromaticRefraction(vec3 viewDirection, vec3 normal, float ior, float chromaticStrength) {
  vec3 refractedR = refract(viewDirection, normal, 1.0 / (ior - chromaticStrength));
  vec3 refractedG = refract(viewDirection, normal, 1.0 / ior);
  vec3 refractedB = refract(viewDirection, normal, 1.0 / (ior + chromaticStrength));
  
  #ifdef ENVMAP_TYPE_CUBE
  return vec3(
    textureCube(envMap, refractedR).r,
    textureCube(envMap, refractedG).g,
    textureCube(envMap, refractedB).b
  );
  #else
  return vec3(0.5);
  #endif
}

void main() {
  #include <clipping_planes_fragment>
  
  vec4 diffuseColor = vec4(diffuse, opacity);
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;
  
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <specularmap_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  
  // Glass-specific calculations
  vec3 viewDirection = normalize(vViewPosition);
  vec3 worldNormal = normalize(vNormal);
  
  // Calculate Fresnel effect
  float fresnelFactor = fresnel(viewDirection, worldNormal, uFresnelPower);
  
  // Base glass color gradient
  vec3 gradientColor = mix(uColor1, uColor2, vUv.y);
  gradientColor = mix(gradientColor, uColor3, fresnelFactor);
  
  // Reflection
  #ifdef ENVMAP_TYPE_CUBE
  vec3 reflectionColor = textureCube(envMap, vReflect).rgb * envMapIntensity;
  #else
  vec3 reflectionColor = vec3(0.5);
  #endif
  
  // Refraction with chromatic aberration
  vec3 refractionColor;
  #ifdef ENVMAP_TYPE_CUBE
  if (uChromaticAberration > 0.0) {
    refractionColor = chromaticRefraction(-viewDirection, worldNormal, uRefraction, uChromaticAberration);
  } else {
    refractionColor = textureCube(envMap, vRefract).rgb;
  }
  refractionColor *= envMapIntensity;
  #else
  refractionColor = vec3(0.3);
  #endif
  
  // Mix reflection and refraction based on Fresnel
  vec3 envColor = mix(refractionColor, reflectionColor, fresnelFactor * uReflectivity);
  
  // Combine with gradient color
  vec3 finalColor = mix(gradientColor, envColor, 0.7);
  
  // Apply transparency
  float finalAlpha = mix(uTransparency, 1.0, fresnelFactor * 0.5);
  
  // Set diffuse color for standard lighting
  diffuseColor.rgb = finalColor;
  diffuseColor.a = finalAlpha;
  
  // Skip transmission_fragment to avoid conflicts
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + 
                       reflectedLight.directSpecular + reflectedLight.indirectSpecular + 
                       totalEmissiveRadiance;
  
  // Add our glass color contribution
  outgoingLight += finalColor * 0.8;
  
  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  
  #include <tonemapping_fragment>
  #include <colorspace_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>
}
`,St=Z({fragment:()=>bt,vertex:()=>Et}),At=`// Glass Sphere Vertex Shader - Refraction & Transparency Effects

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;
varying float vDistortion;

uniform float uTime;
uniform float uSpeed;
uniform float uWaveAmplitude;
uniform float uWaveFrequency;
uniform float uNoiseStrength;
uniform float uDistortion;

// Noise functions for glass distortion
vec3 mod289(vec3 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x) {
  return mod289(((x * 34.0) + 1.0) * x);
}

vec4 taylorInvSqrt(vec4 r) {
  return 1.79284291400159 - 0.85373472095314 * r;
}

float snoise(vec3 v) {
  const vec2 C = vec2(1.0 / 6.0, 1.0 / 3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);

  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);

  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);

  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;

  i = mod289(i);
  vec4 p = permute(permute(permute(
    i.z + vec4(0.0, i1.z, i2.z, 1.0))
    + i.y + vec4(0.0, i1.y, i2.y, 1.0))
    + i.x + vec4(0.0, i1.x, i2.x, 1.0));

  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;

  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);

  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);

  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);

  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);

  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));

  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;

  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);

  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x;
  p1 *= norm.y;
  p2 *= norm.z;
  p3 *= norm.w;

  vec4 m = max(0.6 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1),
    dot(p2, x2), dot(p3, x3)));
}

void main() {
  #include <uv_pars_vertex>
  #include <uv_vertex>
  #include <uv2_pars_vertex>
  #include <uv2_vertex>
  #include <color_pars_vertex>
  #include <color_vertex>
  #include <morphcolor_vertex>
  #include <beginnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <defaultnormal_vertex>
  #include <normal_vertex>
  
  #ifndef FLAT_SHADED
  vNormal = normalize(transformedNormal);
  #ifdef USE_TANGENT
  vTangent = normalize(transformedTangent);
  vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  
  #include <begin_vertex>
  #include <morphtarget_vertex>
  #include <skinning_vertex>
  #include <displacementmap_vertex>
  
  // Pass UV coordinates
  vUv = uv;

  // Calculate time-based animation
  float time = uTime * uSpeed;
  
  // For sphere, use spherical coordinates for better distortion
  float theta = atan(position.z, position.x);
  float phi = acos(position.y / length(position));
  
  // Create waves based on spherical coordinates
  float waveTheta = sin(theta * uWaveFrequency * 2.0 + time) * uWaveAmplitude;
  float wavePhi = cos(phi * uWaveFrequency + time * 1.5) * uWaveAmplitude;
  
  // Add noise for organic glass distortion
  vec3 noisePos = position + vec3(time * 0.1);
  float noise = snoise(noisePos * 0.8) * uNoiseStrength;
  
  // Calculate distortion based on position on sphere
  float distortionAmount = (waveTheta + wavePhi) * uDistortion + noise;
  vDistortion = distortionAmount;
  
  // Apply distortion along normal for sphere
  transformed += normal * distortionAmount;
  
  #include <project_vertex>
  #include <logdepthbuf_vertex>
  #include <clipping_planes_vertex>
  
  vViewPosition = -mvPosition.xyz;
  vPosition = transformed;
  
  // Calculate world position for refraction
  vec4 worldPosition = modelMatrix * vec4(transformed, 1.0);
  vGlassWorldPos = worldPosition.xyz;
  
  // Calculate reflection and refraction vectors
  vec3 worldNormal = normalize(mat3(modelMatrix) * normal);
  vec3 viewVector = normalize(cameraPosition - worldPosition.xyz);
  
  // Reflection vector
  vReflect = reflect(-viewVector, worldNormal);
  
  // Refraction vector with index of refraction for glass (1.5)
  // For sphere, adjust IOR based on curvature
  float ior = 1.5 + sin(theta * 2.0 + time) * 0.1;
  vRefract = refract(-viewVector, worldNormal, 1.0 / ior);
  
  #include <fog_vertex>
  #include <shadowmap_vertex>
}
`,Ot=`// Glass Sphere Fragment Shader - Transparency & Refraction

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

// transmission is already defined by Three.js when TRANSMISSION is enabled
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif

varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif

#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
#include <transmission_pars_fragment>

// Custom uniforms for glass effect
uniform float uTime;
uniform vec3 uColor1;
uniform vec3 uColor2;
uniform vec3 uColor3;
uniform float uTransparency;
uniform float uRefraction;
uniform float uChromaticAberration;
uniform float uFresnelPower;
uniform float uReflectivity;
// envMap and envMapIntensity are provided by Three.js

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;
varying float vDistortion;

// Fresnel calculation
float fresnel(vec3 viewDirection, vec3 normal, float power) {
  return pow(1.0 - abs(dot(viewDirection, normal)), power);
}

// Chromatic aberration for refraction
vec3 chromaticRefraction(vec3 viewDirection, vec3 normal, float ior, float chromaticStrength) {
  vec3 refractedR = refract(viewDirection, normal, 1.0 / (ior - chromaticStrength));
  vec3 refractedG = refract(viewDirection, normal, 1.0 / ior);
  vec3 refractedB = refract(viewDirection, normal, 1.0 / (ior + chromaticStrength));
  
  #ifdef ENVMAP_TYPE_CUBE
  return vec3(
    textureCube(envMap, refractedR).r,
    textureCube(envMap, refractedG).g,
    textureCube(envMap, refractedB).b
  );
  #else
  return vec3(0.5);
  #endif
}

// Caustics simulation for sphere
float caustics(vec3 position, float time) {
  float c1 = sin(position.x * 4.0 + time) * sin(position.y * 4.0 + time * 0.8);
  float c2 = sin(position.z * 3.0 - time * 1.2) * sin(position.x * 3.0 + time);
  return (c1 + c2) * 0.5 + 0.5;
}

void main() {
  #include <clipping_planes_fragment>
  
  vec4 diffuseColor = vec4(diffuse, opacity);
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;
  
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <specularmap_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  
  // Glass-specific calculations
  vec3 viewDirection = normalize(vViewPosition);
  vec3 worldNormal = normalize(vNormal);
  
  // Calculate Fresnel effect
  float fresnelFactor = fresnel(viewDirection, worldNormal, uFresnelPower);
  
  // For sphere, use spherical UV mapping for gradient
  float sphericalU = atan(vPosition.z, vPosition.x) / (2.0 * PI) + 0.5;
  float sphericalV = acos(vPosition.y / length(vPosition)) / PI;
  vec2 sphericalUV = vec2(sphericalU, sphericalV);
  
  // Create color gradient based on spherical coordinates
  vec3 gradientColor = mix(uColor1, uColor2, sphericalUV.y);
  gradientColor = mix(gradientColor, uColor3, pow(fresnelFactor, 1.5));
  
  // Add caustics effect for sphere
  float causticsValue = caustics(vGlassWorldPos, uTime);
  gradientColor += vec3(causticsValue * 0.1);
  
  // Reflection
  #ifdef ENVMAP_TYPE_CUBE
  vec3 reflectionColor = textureCube(envMap, vReflect).rgb * envMapIntensity;
  #else
  vec3 reflectionColor = vec3(0.5);
  #endif
  
  // Refraction with chromatic aberration (enhanced for sphere)
  vec3 refractionColor;
  #ifdef ENVMAP_TYPE_CUBE
  if (uChromaticAberration > 0.0) {
    float chromaticIntensity = uChromaticAberration * (1.0 + vDistortion * 0.5);
    refractionColor = chromaticRefraction(-viewDirection, worldNormal, uRefraction, chromaticIntensity);
  } else {
    refractionColor = textureCube(envMap, vRefract).rgb;
  }
  refractionColor *= envMapIntensity;
  #else
  refractionColor = vec3(0.3);
  #endif
  
  // Mix reflection and refraction based on Fresnel (stronger effect for sphere)
  vec3 envColor = mix(refractionColor, reflectionColor, fresnelFactor * uReflectivity);
  
  // Add inner glow effect for sphere
  float innerGlow = pow(1.0 - abs(dot(viewDirection, worldNormal)), 3.0);
  vec3 glowColor = mix(uColor2, uColor3, innerGlow) * innerGlow * 0.5;
  
  // Combine all effects
  vec3 finalColor = mix(gradientColor, envColor, 0.8) + glowColor;
  
  // Apply transparency with sphere thickness consideration
  float thickness = 1.0 - pow(abs(dot(viewDirection, worldNormal)), 0.5);
  float finalAlpha = mix(uTransparency * thickness, 1.0, fresnelFactor * 0.7);
  
  // Set diffuse color for standard lighting
  diffuseColor.rgb = finalColor;
  diffuseColor.a = finalAlpha;
  
  // Skip transmission_fragment to avoid conflicts
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + 
                       reflectedLight.directSpecular + reflectedLight.indirectSpecular + 
                       totalEmissiveRadiance;
  
  // Add our glass color contribution
  outgoingLight += finalColor * 0.9;
  
  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  
  #include <tonemapping_fragment>
  #include <colorspace_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>
}
`,Dt=Z({fragment:()=>Ot,vertex:()=>At}),Nt=`// Glass WaterPlane Vertex Shader - Liquid Glass Effect

#define STANDARD
varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif
#include <clipping_planes_pars_vertex>
#include <color_pars_vertex>
#include <common>
#include <displacementmap_pars_vertex>
#include <fog_pars_vertex>
#include <logdepthbuf_pars_vertex>
#include <morphtarget_pars_vertex>
#include <shadowmap_pars_vertex>
#include <skinning_pars_vertex>
#include <uv2_pars_vertex>
#include <uv_pars_vertex>

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;
varying float vWaveHeight;
varying vec3 vWaveNormal;

uniform float uTime;
uniform float uSpeed;
uniform float uWaveAmplitude;
uniform float uWaveFrequency;
uniform float uNoiseStrength;
uniform float uDistortion;
uniform float uFlowSpeed;
uniform vec2 uFlowDirection;

// Noise functions for water-like glass distortion
vec3 mod289(vec3 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 mod289(vec4 x) {
  return x - floor(x * (1.0 / 289.0)) * 289.0;
}

vec4 permute(vec4 x) {
  return mod289(((x * 34.0) + 1.0) * x);
}

vec4 taylorInvSqrt(vec4 r) {
  return 1.79284291400159 - 0.85373472095314 * r;
}

float snoise(vec3 v) {
  const vec2 C = vec2(1.0 / 6.0, 1.0 / 3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);

  vec3 i = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);

  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);

  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;

  i = mod289(i);
  vec4 p = permute(permute(permute(
    i.z + vec4(0.0, i1.z, i2.z, 1.0))
    + i.y + vec4(0.0, i1.y, i2.y, 1.0))
    + i.x + vec4(0.0, i1.x, i2.x, 1.0));

  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;

  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);

  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);

  vec4 x = x_ * ns.x + ns.yyyy;
  vec4 y = y_ * ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);

  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);

  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));

  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;

  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);

  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x;
  p1 *= norm.y;
  p2 *= norm.z;
  p3 *= norm.w;

  vec4 m = max(0.6 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1),
    dot(p2, x2), dot(p3, x3)));
}

// Water wave function
vec3 waterWave(vec2 pos, float time) {
  // Flow effect
  vec2 flowPos = pos + uFlowDirection * time * uFlowSpeed;
  
  // Multiple wave layers for realistic water
  float wave1 = sin(flowPos.x * uWaveFrequency + time) * cos(flowPos.y * uWaveFrequency * 0.8 + time * 0.7);
  float wave2 = sin(flowPos.x * uWaveFrequency * 1.7 - time * 1.3) * sin(flowPos.y * uWaveFrequency * 1.3 + time);
  float wave3 = cos(flowPos.x * uWaveFrequency * 0.5 + time * 0.5) * sin(flowPos.y * uWaveFrequency * 0.6 - time * 0.8);
  
  // Combine waves
  float height = (wave1 * 0.5 + wave2 * 0.3 + wave3 * 0.2) * uWaveAmplitude;
  
  // Calculate wave normals
  float dx = cos(flowPos.x * uWaveFrequency + time) * uWaveFrequency * 0.5 * uWaveAmplitude;
  float dz = -sin(flowPos.y * uWaveFrequency * 0.8 + time * 0.7) * uWaveFrequency * 0.8 * 0.5 * uWaveAmplitude;
  
  return vec3(dx, height, dz);
}

void main() {
  #include <uv_pars_vertex>
  #include <uv_vertex>
  #include <uv2_pars_vertex>
  #include <uv2_vertex>
  #include <color_pars_vertex>
  #include <color_vertex>
  #include <morphcolor_vertex>
  #include <beginnormal_vertex>
  #include <morphnormal_vertex>
  #include <skinbase_vertex>
  #include <skinnormal_vertex>
  #include <defaultnormal_vertex>
  #include <normal_vertex>
  
  // Pass UV coordinates
  vUv = uv;

  // Calculate time-based animation
  float time = uTime * uSpeed;
  
  // Calculate water waves
  vec3 waveData = waterWave(position.xz, time);
  float waveHeight = waveData.y;
  vec2 waveGradient = waveData.xz;
  
  // Add noise for organic water movement
  vec3 noisePos = vec3(position.x, position.y, position.z) + vec3(time * 0.05);
  float noise = snoise(noisePos * 1.2) * uNoiseStrength * 0.5;
  
  // Store wave height for fragment shader
  vWaveHeight = waveHeight + noise;
  
  // Calculate perturbed normal for water surface
  vec3 waveNormal = normalize(vec3(-waveGradient.x, 1.0, -waveGradient.y));
  vWaveNormal = waveNormal;
  
  // Blend original normal with wave normal
  vec3 blendedNormal = normalize(mix(normal, waveNormal, 0.7));
  
  #ifndef FLAT_SHADED
  vNormal = normalize(mat3(modelViewMatrix) * blendedNormal);
  #ifdef USE_TANGENT
  vTangent = normalize(transformedTangent);
  vBitangent = normalize(cross(vNormal, vTangent) * tangent.w);
  #endif
  #endif
  
  #include <begin_vertex>
  #include <morphtarget_vertex>
  #include <skinning_vertex>
  #include <displacementmap_vertex>
  
  // Apply wave displacement and additional distortion
  transformed.y += waveHeight + noise;
  transformed += blendedNormal * uDistortion * noise;
  
  #include <project_vertex>
  #include <logdepthbuf_vertex>
  #include <clipping_planes_vertex>
  
  vViewPosition = -mvPosition.xyz;
  vPosition = transformed;
  
  // Calculate world position for refraction
  vec4 worldPosition = modelMatrix * vec4(transformed, 1.0);
  vGlassWorldPos = worldPosition.xyz;
  
  // Calculate reflection and refraction vectors with wave normal
  vec3 worldNormal = normalize(mat3(modelMatrix) * blendedNormal);
  vec3 viewVector = normalize(cameraPosition - worldPosition.xyz);
  
  // Reflection vector
  vReflect = reflect(-viewVector, worldNormal);
  
  // Refraction vector with varying IOR for water effect
  float ior = 1.33 + sin(time + position.x * 2.0) * 0.1; // Water IOR ~1.33
  vRefract = refract(-viewVector, worldNormal, 1.0 / ior);
  
  #include <fog_vertex>
  #include <shadowmap_vertex>
}
`,Rt=`// Glass WaterPlane Fragment Shader - Liquid Glass Effect

#define STANDARD
#ifdef PHYSICAL
#define REFLECTIVITY
#define CLEARCOAT
#define TRANSMISSION
#endif

uniform vec3 diffuse;
uniform vec3 emissive;
uniform float roughness;
uniform float metalness;
uniform float opacity;

// transmission is already defined by Three.js when TRANSMISSION is enabled
#ifdef REFLECTIVITY
uniform float reflectivity;
#endif
#ifdef CLEARCOAT
uniform float clearcoat;
uniform float clearcoatRoughness;
#endif
#ifdef USE_SHEEN
uniform vec3 sheen;
#endif

varying vec3 vViewPosition;
#ifndef FLAT_SHADED
#ifdef USE_TANGENT
varying vec3 vTangent;
varying vec3 vBitangent;
#endif
#endif

#include <alphamap_pars_fragment>
#include <aomap_pars_fragment>
#include <color_pars_fragment>
#include <common>
#include <dithering_pars_fragment>
#include <emissivemap_pars_fragment>
#include <lightmap_pars_fragment>
#include <map_pars_fragment>
#include <packing>
#include <uv2_pars_fragment>
#include <uv_pars_fragment>
#include <bsdfs>
#include <bumpmap_pars_fragment>
#include <clearcoat_pars_fragment>
#include <clipping_planes_pars_fragment>
#include <envmap_common_pars_fragment>
#include <envmap_pars_fragment>
#include <envmap_physical_pars_fragment>
#include <fog_pars_fragment>
#include <lights_pars_begin>
#include <lights_physical_pars_fragment>
#include <logdepthbuf_pars_fragment>
#include <metalnessmap_pars_fragment>
#include <normalmap_pars_fragment>
#include <roughnessmap_pars_fragment>
#include <shadowmap_pars_fragment>
#include <transmission_pars_fragment>

// Custom uniforms for liquid glass effect
uniform float uTime;
uniform vec3 uColor1;
uniform vec3 uColor2;
uniform vec3 uColor3;
uniform float uTransparency;
uniform float uRefraction;
uniform float uChromaticAberration;
uniform float uFresnelPower;
uniform float uReflectivity;
// envMap and envMapIntensity are provided by Three.js
uniform float uLiquidEffect;
uniform float uFoamIntensity;

varying vec2 vUv;
varying vec3 vPosition;
varying vec3 vNormal;
varying vec3 vGlassWorldPos;
varying vec3 vReflect;
varying vec3 vRefract;
varying float vWaveHeight;
varying vec3 vWaveNormal;

// Fresnel calculation
float fresnel(vec3 viewDirection, vec3 normal, float power) {
  return pow(1.0 - abs(dot(viewDirection, normal)), power);
}

// Chromatic aberration for refraction
vec3 chromaticRefraction(vec3 viewDirection, vec3 normal, float ior, float chromaticStrength) {
  vec3 refractedR = refract(viewDirection, normal, 1.0 / (ior - chromaticStrength));
  vec3 refractedG = refract(viewDirection, normal, 1.0 / ior);
  vec3 refractedB = refract(viewDirection, normal, 1.0 / (ior + chromaticStrength));
  
  #ifdef ENVMAP_TYPE_CUBE
  vec3 result = vec3(
    textureCube(envMap, refractedR).r,
    textureCube(envMap, refractedG).g,
    textureCube(envMap, refractedB).b
  );
  
  // Add distortion based on wave height
  float distortion = vWaveHeight * 0.1;
  result = mix(result, textureCube(envMap, refractedG + vec3(distortion)).rgb, 0.3);
  #else
  vec3 result = vec3(0.5);
  #endif
  
  return result;
}

// Foam effect for water surface
float foam(vec2 uv, float waveHeight, float time) {
  float foamThreshold = 0.3;
  float foamAmount = smoothstep(foamThreshold - 0.1, foamThreshold + 0.1, abs(waveHeight));
  
  // Add foam texture pattern
  float foamPattern = sin(uv.x * 40.0 + time) * cos(uv.y * 30.0 - time * 0.5);
  foamPattern += sin(uv.x * 25.0 - time * 0.8) * sin(uv.y * 35.0 + time);
  foamPattern = clamp(foamPattern * 0.5 + 0.5, 0.0, 1.0);
  
  return foamAmount * foamPattern;
}

// Caustics for underwater effect
vec3 caustics(vec3 position, float time) {
  float c1 = sin(position.x * 6.0 + time * 1.5) * sin(position.z * 6.0 + time);
  float c2 = cos(position.x * 4.0 - time) * cos(position.z * 5.0 + time * 1.2);
  float c3 = sin((position.x + position.z) * 3.0 + time * 0.8);
  
  float causticPattern = (c1 + c2 + c3) / 3.0;
  causticPattern = pow(max(0.0, causticPattern), 2.0);
  
  return vec3(causticPattern) * vec3(0.3, 0.6, 1.0);
}

void main() {
  #include <clipping_planes_fragment>
  
  vec4 diffuseColor = vec4(diffuse, opacity);
  ReflectedLight reflectedLight = ReflectedLight(vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
  vec3 totalEmissiveRadiance = emissive;
  
  #include <logdepthbuf_fragment>
  #include <map_fragment>
  #include <color_fragment>
  #include <alphamap_fragment>
  #include <alphatest_fragment>
  #include <specularmap_fragment>
  #include <roughnessmap_fragment>
  #include <metalnessmap_fragment>
  #include <normal_fragment_begin>
  #include <normal_fragment_maps>
  #include <clearcoat_normal_fragment_begin>
  #include <clearcoat_normal_fragment_maps>
  #include <emissivemap_fragment>
  
  // Use wave normal for more accurate water surface
  vec3 viewDirection = normalize(vViewPosition);
  vec3 worldNormal = normalize(vWaveNormal);
  
  // Calculate Fresnel effect
  float fresnelFactor = fresnel(viewDirection, worldNormal, uFresnelPower);
  
  // Water color gradient with depth effect
  float depth = 1.0 - abs(vWaveHeight) * 2.0;
  vec3 shallowColor = mix(uColor1, uColor2, vUv.y);
  vec3 deepColor = mix(uColor2, uColor3, depth);
  vec3 gradientColor = mix(shallowColor, deepColor, fresnelFactor);
  
  // Add foam effect
  float foamAmount = foam(vUv, vWaveHeight, uTime) * uFoamIntensity;
  vec3 foamColor = vec3(1.0, 1.0, 1.0);
  gradientColor = mix(gradientColor, foamColor, foamAmount);
  
  // Reflection
  #ifdef ENVMAP_TYPE_CUBE
  vec3 reflectionColor = textureCube(envMap, vReflect).rgb * envMapIntensity;
  
  // Add slight blur to reflection for water effect
  vec3 blurredReflection = reflectionColor;
  for (int i = 0; i < 4; i++) {
    vec3 offset = vec3(
      sin(float(i) * 2.0) * 0.01,
      0.0,
      cos(float(i) * 2.0) * 0.01
    );
    blurredReflection += textureCube(envMap, vReflect + offset).rgb * envMapIntensity;
  }
  blurredReflection /= 5.0;
  reflectionColor = mix(reflectionColor, blurredReflection, uLiquidEffect);
  #else
  vec3 reflectionColor = vec3(0.5);
  #endif
  
  // Refraction with chromatic aberration (stronger for water)
  vec3 refractionColor;
  #ifdef ENVMAP_TYPE_CUBE
  if (uChromaticAberration > 0.0) {
    float waterIOR = 1.33 + vWaveHeight * 0.1;
    refractionColor = chromaticRefraction(-viewDirection, worldNormal, waterIOR, uChromaticAberration * 1.5);
  } else {
    refractionColor = textureCube(envMap, vRefract).rgb;
  }
  refractionColor *= envMapIntensity;
  #else
  refractionColor = vec3(0.3);
  #endif
  
  // Add caustics to refraction
  vec3 causticsColor = caustics(vGlassWorldPos, uTime);
  refractionColor += causticsColor * 0.3 * uLiquidEffect;
  
  // Mix reflection and refraction based on Fresnel and wave
  float reflectionMix = fresnelFactor * uReflectivity * (1.0 + abs(vWaveHeight));
  vec3 envColor = mix(refractionColor, reflectionColor, clamp(reflectionMix, 0.0, 1.0));
  
  // Combine all effects
  vec3 finalColor = mix(gradientColor, envColor, 0.85);
  
  // Apply transparency with wave variation
  float waveAlpha = 1.0 - abs(vWaveHeight) * 0.3;
  float finalAlpha = mix(uTransparency * waveAlpha, 1.0, fresnelFactor * 0.6 + foamAmount * 0.4);
  
  // Set diffuse color for standard lighting
  diffuseColor.rgb = finalColor;
  diffuseColor.a = finalAlpha;
  
  // Skip transmission_fragment to avoid conflicts
  
  vec3 outgoingLight = reflectedLight.directDiffuse + reflectedLight.indirectDiffuse + 
                       reflectedLight.directSpecular + reflectedLight.indirectSpecular + 
                       totalEmissiveRadiance;
  
  // Add our liquid glass color contribution
  outgoingLight += finalColor * 0.95;
  
  gl_FragColor = vec4(outgoingLight, diffuseColor.a);
  
  #include <tonemapping_fragment>
  #include <colorspace_fragment>
  #include <fog_fragment>
  #include <premultiplied_alpha_fragment>
  #include <dithering_fragment>
}
`,Lt=Z({fragment:()=>Rt,vertex:()=>Nt}),Ut=Z({plane:()=>St,sphere:()=>Dt,waterPlane:()=>Lt}),It=Z({cosmic:()=>wt,defaults:()=>ot,glass:()=>Ut,positionMix:()=>vt});const Ft={uniforms:{tDiffuse:{value:null},shape:{value:1},radius:{value:2},rotateR:{value:Math.PI/12*1},rotateG:{value:Math.PI/12*2},rotateB:{value:Math.PI/12*3},scatter:{value:1},width:{value:20},height:{value:20},blending:{value:1},blendingMode:{value:1},greyscale:{value:!1},disable:{value:!1}},vertexShader:`
    varying vec2 vUV;
    varying vec3 vPosition;

    void main() {
      vUV = uv;
      vPosition = position;
      gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
    }
  `,fragmentShader:`
    #define SQRT2_MINUS_ONE 0.41421356
    #define SQRT2_HALF_MINUS_ONE 0.20710678
    #define PI2 6.28318531
    #define SHAPE_DOT 1
    #define SHAPE_ELLIPSE 2
    #define SHAPE_LINE 3
    #define SHAPE_SQUARE 4
    #define BLENDING_LINEAR 1
    #define BLENDING_MULTIPLY 2
    #define BLENDING_ADD 3
    #define BLENDING_LIGHTER 4
    #define BLENDING_DARKER 5

    uniform sampler2D tDiffuse;
    uniform float radius;
    uniform float rotateR;
    uniform float rotateG;
    uniform float rotateB;
    uniform float scatter;
    uniform float width;
    uniform float height;
    uniform int shape;
    uniform bool disable;
    uniform float blending;
    uniform int blendingMode;
    uniform bool greyscale;

    varying vec2 vUV;
    varying vec3 vPosition;

    const int samples = 8;

    float blend(float a, float b, float t) {
      return a * (1.0 - t) + b * t;
    }

    float hypot2(float x, float y) {
      return sqrt(x * x + y * y);
    }

    float rand(vec2 seed) {
      return fract(sin(dot(seed.xy, vec2(12.9898, 78.233))) * 43758.5453);
    }

    float distanceToDotRadius(float channel, vec2 coord, vec2 normal, vec2 p, float angle, float rad_max) {
      float dist = hypot2(coord.x - p.x, coord.y - p.y);
      float rad = channel;

      if (shape == SHAPE_DOT) {
        rad = pow(abs(rad), 1.125) * rad_max;
      } else if (shape == SHAPE_ELLIPSE) {
        rad = pow(abs(rad), 1.125) * rad_max;

        if (dist != 0.0) {
          float dot_p = abs((p.x - coord.x) / dist * normal.x + (p.y - coord.y) / dist * normal.y);
          dist = (dist * (1.0 - SQRT2_HALF_MINUS_ONE)) + dot_p * dist * SQRT2_MINUS_ONE;
        }
      } else if (shape == SHAPE_LINE) {
        rad = pow(abs(rad), 1.5) * rad_max;
        float dot_p = (p.x - coord.x) * normal.x + (p.y - coord.y) * normal.y;
        dist = hypot2(normal.x * dot_p, normal.y * dot_p);
      } else if (shape == SHAPE_SQUARE) {
        float theta = atan(p.y - coord.y, p.x - coord.x) - angle;
        float sin_t = abs(sin(theta));
        float cos_t = abs(cos(theta));
        rad = pow(abs(rad), 1.4);
        rad = rad_max * (rad + ((sin_t > cos_t) ? rad - sin_t * rad : rad - cos_t * rad));
      }

      return rad - dist;
    }

    struct Cell {
      vec2 normal;
      vec2 p1;
      vec2 p2;
      vec2 p3;
      vec2 p4;
      float samp2;
      float samp1;
      float samp3;
      float samp4;
    };

    vec4 getSample(vec2 point) {
      vec4 tex = texture2D(tDiffuse, vec2(point.x / width, point.y / height));
      float base = rand(vec2(floor(point.x), floor(point.y))) * PI2;
      float step = PI2 / float(samples);
      float dist = radius * 0.0;

      for (int i = 0; i < samples; ++i) {
        float r = base + step * float(i);
        vec2 coord = point + vec2(cos(r) * dist, sin(r) * dist);
        tex += texture2D(tDiffuse, vec2(coord.x / width, coord.y / height));
      }

      tex /= float(samples) + 1.0;
      return tex;
    }

    float getDotColour(Cell c, vec2 p, int channel, float angle, float aa) {
      float dist_c_1;
      float dist_c_2;
      float dist_c_3;
      float dist_c_4;
      float res;

      if (channel == 0) {
        c.samp1 = getSample(c.p1).r;
        c.samp2 = getSample(c.p2).r;
        c.samp3 = getSample(c.p3).r;
        c.samp4 = getSample(c.p4).r;
      } else if (channel == 1) {
        c.samp1 = getSample(c.p1).g;
        c.samp2 = getSample(c.p2).g;
        c.samp3 = getSample(c.p3).g;
        c.samp4 = getSample(c.p4).g;
      } else {
        c.samp1 = getSample(c.p1).b;
        c.samp2 = getSample(c.p2).b;
        c.samp3 = getSample(c.p3).b;
        c.samp4 = getSample(c.p4).b;
      }

      dist_c_1 = distanceToDotRadius(c.samp1, c.p1, c.normal, p, angle, radius);
      dist_c_2 = distanceToDotRadius(c.samp2, c.p2, c.normal, p, angle, radius);
      dist_c_3 = distanceToDotRadius(c.samp3, c.p3, c.normal, p, angle, radius);
      dist_c_4 = distanceToDotRadius(c.samp4, c.p4, c.normal, p, angle, radius);
      res = (dist_c_1 > 0.0) ? clamp(dist_c_1 / aa, 0.0, 1.0) : 0.0;
      res += (dist_c_2 > 0.0) ? clamp(dist_c_2 / aa, 0.0, 1.0) : 0.0;
      res += (dist_c_3 > 0.0) ? clamp(dist_c_3 / aa, 0.0, 1.0) : 0.0;
      res += (dist_c_4 > 0.0) ? clamp(dist_c_4 / aa, 0.0, 1.0) : 0.0;
      res = clamp(res, 0.0, 1.0);
      return res;
    }

    Cell getReferenceCell(vec2 p, vec2 origin, float grid_angle, float step) {
      Cell c;

      vec2 n = vec2(cos(grid_angle), sin(grid_angle));
      float threshold = step * 0.5;
      float dot_normal = n.x * (p.x - origin.x) + n.y * (p.y - origin.y);
      float dot_line = -n.y * (p.x - origin.x) + n.x * (p.y - origin.y);
      vec2 offset = vec2(n.x * dot_normal, n.y * dot_normal);
      float offset_normal = mod(hypot2(offset.x, offset.y), step);
      float normal_dir = (dot_normal < 0.0) ? 1.0 : -1.0;
      float normal_scale = ((offset_normal < threshold) ? -offset_normal : step - offset_normal) * normal_dir;
      float offset_line = mod(hypot2((p.x - offset.x) - origin.x, (p.y - offset.y) - origin.y), step);
      float line_dir = (dot_line < 0.0) ? 1.0 : -1.0;
      float line_scale = ((offset_line < threshold) ? -offset_line : step - offset_line) * line_dir;

      c.normal = n;
      c.p1.x = p.x - n.x * normal_scale + n.y * line_scale;
      c.p1.y = p.y - n.y * normal_scale - n.x * line_scale;

      if (scatter != 0.0) {
        float off_mag = scatter * threshold * 0.5;
        float off_angle = rand(vec2(floor(c.p1.x), floor(c.p1.y))) * PI2;
        c.p1.x += cos(off_angle) * off_mag;
        c.p1.y += sin(off_angle) * off_mag;
      }

      float normal_step = normal_dir * ((offset_normal < threshold) ? step : -step);
      float line_step = line_dir * ((offset_line < threshold) ? step : -step);
      c.p2.x = c.p1.x - n.x * normal_step;
      c.p2.y = c.p1.y - n.y * normal_step;
      c.p3.x = c.p1.x + n.y * line_step;
      c.p3.y = c.p1.y - n.x * line_step;
      c.p4.x = c.p1.x - n.x * normal_step + n.y * line_step;
      c.p4.y = c.p1.y - n.y * normal_step - n.x * line_step;

      return c;
    }

    float blendColour(float a, float b, float t) {
      if (blendingMode == BLENDING_LINEAR) {
        return blend(a, b, 1.0 - t);
      } else if (blendingMode == BLENDING_ADD) {
        return blend(a, min(1.0, a + b), t);
      } else if (blendingMode == BLENDING_MULTIPLY) {
        return blend(a, max(0.0, a * b), t);
      } else if (blendingMode == BLENDING_LIGHTER) {
        return blend(a, max(a, b), t);
      } else if (blendingMode == BLENDING_DARKER) {
        return blend(a, min(a, b), t);
      }

      return blend(a, b, 1.0 - t);
    }

    void main() {
      if (!disable) {
        vec2 p = vec2(vUV.x * width, vUV.y * height) - vec2(vPosition.x, vPosition.y) * 3.0;
        vec2 origin = vec2(0.0, 0.0);
        float aa = (radius < 2.5) ? radius * 0.5 : 1.25;

        Cell cell_r = getReferenceCell(p, origin, rotateR, radius);
        Cell cell_g = getReferenceCell(p, origin, rotateG, radius);
        Cell cell_b = getReferenceCell(p, origin, rotateB, radius);
        float r = getDotColour(cell_r, p, 0, rotateR, aa);
        float g = getDotColour(cell_g, p, 1, rotateG, aa);
        float b = getDotColour(cell_b, p, 2, rotateB, aa);

        vec4 colour = texture2D(tDiffuse, vUV);

        if (colour.r == 0.0) {
          r = 0.0;
        } else {
          r = blendColour(r, colour.r, blending);
        }

        if (colour.g == 0.0) {
          g = 0.0;
        } else {
          g = blendColour(g, colour.g, blending);
        }

        if (colour.b == 0.0) {
          b = 0.0;
        } else {
          b = blendColour(b, colour.b, blending);
        }

        if (greyscale) {
          r = g = b = (r + b + g) / 3.0;
        }

        vec4 vR;
        vec4 vG;
        vec4 vB;

        if (r == 0.0 && colour.r == 0.0) {
          vR = vec4(0.0, 0.0, 0.0, 0.0);
        } else {
          vR = vec4(r, 0.0, 0.0, 1.0);
        }

        if (g == 0.0 && colour.g == 0.0) {
          vG = vec4(0.0, 0.0, 0.0, 0.0);
        } else {
          vG = vec4(0.0, g, 0.0, 1.0);
        }

        if (b == 0.0 && colour.b == 0.0) {
          vB = vec4(0.0, 0.0, 0.0, 0.0);
        } else {
          vB = vec4(0.0, 0.0, b, 1.0);
        }

        gl_FragColor = vR + vG + vB;
      } else {
        gl_FragColor = texture2D(tDiffuse, vUV);
      }
    }
  `},Mt=1,Ht=14,sn={zoom:1,distance:14},ln={zoom:5,distance:14},kt={city:"city.hdr",dawn:"dawn.hdr",lobby:"lobby.hdr"},Yt=["uTime","uSpeed","uStrength","uDensity","uFrequency","uAmplitude","rangeStart","rangeEnd","loopDuration","positionX","positionY","positionZ","rotationX","rotationY","rotationZ","reflection","cAzimuthAngle","cPolarAngle","cDistance","cameraZoom","brightness","grainBlending","fov","pixelDensity"],cn=new Map;W.install({THREE:Nn});function Vt(){const o=Rn;o.uv2_pars_vertex=o.uv2_pars_vertex??"",o.uv2_vertex=o.uv2_vertex??"",o.uv2_pars_fragment=o.uv2_pars_fragment??"",o.encodings_fragment=o.encodings_fragment??o.colorspace_fragment??""}function Bt(o){switch(o){case"sphere":return new Sn(1,64);case"waterPlane":return new Be(10,10,192,192);default:return new Be(10,10,1,192)}}function Zt(o,e){return It[o][e]}function qt(o,e,n){const t=`${e.endsWith("/")?e:`${e}/`}${kt[n]}`,a=cn.get(t);if(a)return a;const s=new Promise((l,f)=>{new Dn().load(t,m=>{const p=o.fromEquirectangular(m);m.dispose(),l(p)},void 0,f)});return cn.set(t,s),s}function Gt(o){const e=o.shader==="glass";return new An({userData:{},metalness:e?0:.2,roughness:e?.1:1-o.reflection,side:On,wireframe:o.wireframe,transparent:e,opacity:e?.3:1,transmission:e?.9:0,thickness:e?.5:0,clearcoat:e?1:0,clearcoatRoughness:0,ior:1.5,envMapIntensity:1})}var Wt=class{constructor(e,n){this.renderer=null,this.scene=null,this.camera=null,this.mesh=null,this.material=null,this.clock=null,this.ambientLight=null,this.axisHelper=null,this.cameraControls=null,this.pmremGenerator=null,this.composer=null,this.grainPass=null,this.environmentTarget=null,this.environmentKey="",this.environmentRequestId=0,this.animationId=0,this.resizeObserver=null,this.shaderUniforms=null,this.container=e,this.options=an(n),this.currentOptions={...this.options},this.currentColors={color1:ne(this.options.color1),color2:ne(this.options.color2),color3:ne(this.options.color3)},Vt(),this.initScene(),this.resizeObserver=new ResizeObserver(()=>this.handleResize()),this.resizeObserver.observe(this.container)}getOptions(){return{...this.options}}update(e){const n=an({...this.options,...e,onCameraUpdate:e.onCameraUpdate??this.options.onCameraUpdate}),t=this.options;this.options=n;const a=t.type!==n.type||t.shader!==n.shader||t.preserveDrawingBuffer!==n.preserveDrawingBuffer||t.powerPreference!==n.powerPreference;if(n.enableTransition||(this.currentOptions={...n},this.currentColors={color1:ne(n.color1),color2:ne(n.color2),color3:ne(n.color3)}),a){this.rebuild();return}this.syncRendererState(),this.syncLighting(),this.syncAxisHelper(),this.syncCameraControls(this.options.enableTransition),this.applyCurrentState()}dispose(){var e,n,t,a,s,l;this.animationId&&(cancelAnimationFrame(this.animationId),this.animationId=0),(e=this.resizeObserver)==null||e.disconnect(),this.resizeObserver=null,(n=this.cameraControls)==null||n.dispose(),this.cameraControls=null,this.mesh&&(this.mesh.geometry.dispose(),(t=this.material)==null||t.dispose(),this.mesh.removeFromParent(),this.mesh=null,this.material=null),this.axisHelper&&(this.axisHelper.removeFromParent(),this.axisHelper=null),(a=this.pmremGenerator)==null||a.dispose(),this.pmremGenerator=null,(s=this.grainPass)==null||s.dispose(),this.grainPass=null,(l=this.composer)==null||l.dispose(),this.composer=null,this.renderer&&(this.renderer.dispose(),this.renderer.domElement.remove(),this.renderer=null),this.scene=null,this.camera=null,this.clock=null,this.ambientLight=null,this.shaderUniforms=null}rebuild(){const e=this.resizeObserver;this.resizeObserver=null,this.dispose(),this.resizeObserver=e,this.currentOptions={...this.options},this.currentColors={color1:ne(this.options.color1),color2:ne(this.options.color2),color3:ne(this.options.color3)},this.initScene()}initScene(){const e=this.container.clientWidth,n=this.container.clientHeight;if(!e||!n)return;this.renderer=new pn({antialias:!0,alpha:!0,preserveDrawingBuffer:this.options.preserveDrawingBuffer,powerPreference:this.options.powerPreference}),this.renderer.setPixelRatio(Math.min(window.devicePixelRatio,this.currentOptions.pixelDensity)),this.renderer.setSize(e,n),this.renderer.domElement.style.display="block",this.renderer.domElement.style.width="100%",this.renderer.domElement.style.height="100%",this.container.append(this.renderer.domElement),this.scene=new hn,this.camera=new _n(this.currentOptions.fov,e/n,.1,100),this.clock=new yn,this.pmremGenerator=new xn(this.renderer),this.ambientLight=new Cn(16777215,0),this.scene.add(this.ambientLight),this.mountMesh(),this.syncLighting(),this.syncAxisHelper(),this.syncCameraControls(!1),this.syncPostProcessing(),this.applyCurrentState();const t=()=>{var s,l,f;this.animationId=requestAnimationFrame(t);const a=((s=this.clock)==null?void 0:s.getDelta())??0;this.options.enableTransition&&this.updateTransitionState(a),(l=this.cameraControls)==null||l.update(a),this.shaderUniforms&&(this.shaderUniforms.uTime.value=this.getAnimatedTime()),this.applyCurrentState(),this.composer?this.composer.render(a):(f=this.renderer)==null||f.render(this.scene,this.camera)};t()}mountMesh(){if(!this.scene)return;const e=Bt(this.options.type),n=Gt(this.options),t=Zt(this.options.shader,this.options.type),a=this.createUniforms();n.onBeforeCompile=s=>{s.uniforms={...s.uniforms,...a},s.vertexShader=t.vertex,s.fragmentShader=t.fragment,this.shaderUniforms=s.uniforms},this.material=n,this.mesh=new Pn(e,n),this.scene.add(this.mesh)}createUniforms(){return{uTime:{value:this.currentOptions.uTime},uSpeed:{value:this.currentOptions.uSpeed},uLoadingTime:{value:1},uNoiseDensity:{value:this.currentOptions.uDensity},uNoiseStrength:{value:this.currentOptions.uStrength},uFrequency:{value:this.currentOptions.uFrequency},uAmplitude:{value:this.currentOptions.uAmplitude},uIntensity:{value:.5},uLoop:{value:this.currentOptions.loop?1:0},uLoopDuration:{value:this.currentOptions.loopDuration},uC1r:{value:this.currentColors.color1[0]},uC1g:{value:this.currentColors.color1[1]},uC1b:{value:this.currentColors.color1[2]},uC2r:{value:this.currentColors.color2[0]},uC2g:{value:this.currentColors.color2[1]},uC2b:{value:this.currentColors.color2[2]},uC3r:{value:this.currentColors.color3[0]},uC3g:{value:this.currentColors.color3[1]},uC3b:{value:this.currentColors.color3[2]},uColor1:{value:new ce(this.currentOptions.color1)},uColor2:{value:new ce(this.currentOptions.color2)},uColor3:{value:new ce(this.currentOptions.color3)},uTransparency:{value:.1},uRefraction:{value:1.5},uChromaticAberration:{value:.1},uFresnelPower:{value:2},uReflectivity:{value:.9},uWaveAmplitude:{value:.02},uWaveFrequency:{value:5},uDistortion:{value:.1},uFlowSpeed:{value:.1},uFlowDirection:{value:new Tn(1,.5)},uLiquidEffect:{value:.5},uFoamIntensity:{value:.3}}}updateTransitionState(e){const n=this.options.smoothTime;for(const t of Yt)this.currentOptions[t]=Ae(this.currentOptions[t],this.options[t],n,e);for(const t of["color1","color2","color3"])this.currentColors[t]=qn(this.currentColors[t],ne(this.options[t]),n,e),this.currentOptions[t]=Bn(this.currentColors[t]);for(const t of["animate","range","loop","wireframe","lightType","envPreset","grain","toggleAxis","zoomOut","hoverState","enableCameraControls","enableCameraUpdate","preserveDrawingBuffer","powerPreference","envBasePath","onCameraUpdate"])this.currentOptions[t]=this.options[t]}applyCurrentState(){if(!(!this.mesh||!this.camera)){if(this.syncClock(),this.syncPostProcessing(),this.mesh.position.set(this.currentOptions.positionX,this.currentOptions.positionY,this.currentOptions.positionZ),this.mesh.rotation.set(Ce(this.currentOptions.rotationX),Ce(this.currentOptions.rotationY),Ce(this.currentOptions.rotationZ)),this.material){const e=this.currentOptions.shader==="glass";this.material.roughness=e?.1:1-this.currentOptions.reflection,this.material.metalness=e?0:.2,this.material.wireframe=this.currentOptions.wireframe,this.material.transparent=e,this.material.opacity=e?.3:1,this.material.transmission=e?.9:0,this.material.thickness=e?.5:0,this.material.clearcoat=e?1:0,this.material.clearcoatRoughness=0,this.material.ior=1.5}this.shaderUniforms&&(this.shaderUniforms.uSpeed.value=this.currentOptions.uSpeed,this.shaderUniforms.uNoiseDensity.value=this.currentOptions.uDensity,this.shaderUniforms.uNoiseStrength.value=this.currentOptions.uStrength,this.shaderUniforms.uFrequency.value=this.currentOptions.uFrequency,this.shaderUniforms.uAmplitude.value=this.currentOptions.uAmplitude,this.shaderUniforms.uLoop.value=this.currentOptions.loop?1:0,this.shaderUniforms.uLoopDuration.value=this.currentOptions.loopDuration,this.shaderUniforms.uC1r.value=this.currentColors.color1[0],this.shaderUniforms.uC1g.value=this.currentColors.color1[1],this.shaderUniforms.uC1b.value=this.currentColors.color1[2],this.shaderUniforms.uC2r.value=this.currentColors.color2[0],this.shaderUniforms.uC2g.value=this.currentColors.color2[1],this.shaderUniforms.uC2b.value=this.currentColors.color2[2],this.shaderUniforms.uC3r.value=this.currentColors.color3[0],this.shaderUniforms.uC3g.value=this.currentColors.color3[1],this.shaderUniforms.uC3b.value=this.currentColors.color3[2],this.shaderUniforms.uColor1&&(this.shaderUniforms.uColor1.value=new ce(this.currentOptions.color1)),this.shaderUniforms.uColor2&&(this.shaderUniforms.uColor2.value=new ce(this.currentOptions.color2)),this.shaderUniforms.uColor3&&(this.shaderUniforms.uColor3.value=new ce(this.currentOptions.color3))),this.camera.fov=this.currentOptions.fov,this.camera.updateProjectionMatrix(),this.ambientLight&&(this.ambientLight.intensity=this.currentOptions.lightType==="3d"?this.currentOptions.brightness*Math.PI:.4),this.grainPass&&(this.grainPass.enabled=this.currentOptions.grain,this.grainPass.uniforms.disable.value=!this.currentOptions.grain,this.grainPass.uniforms.blending.value=this.currentOptions.grainBlending)}}syncRendererState(){var n;if(!this.renderer||!this.camera)return;const e=Math.min(window.devicePixelRatio,this.currentOptions.pixelDensity);this.renderer.setPixelRatio(e),(n=this.composer)==null||n.setPixelRatio(e),this.camera.fov=this.currentOptions.fov,this.camera.updateProjectionMatrix()}syncLighting(){if(!(!this.scene||!this.pmremGenerator)){if(this.options.lightType==="env"){const e=`${this.options.envBasePath}|${this.options.envPreset}`;if(this.environmentKey===e&&this.environmentTarget){this.scene.environment=this.environmentTarget.texture;return}this.environmentKey=e;const n=++this.environmentRequestId;qt(this.pmremGenerator,this.options.envBasePath,this.options.envPreset).then(t=>{!this.scene||n!==this.environmentRequestId||this.options.lightType!=="env"||(this.environmentTarget=t,this.scene.environment=t.texture)}).catch(()=>{!this.scene||n!==this.environmentRequestId||(this.scene.environment=null)});return}this.environmentKey="",this.environmentRequestId+=1,this.scene.environment=null}}syncAxisHelper(){this.scene&&(this.options.toggleAxis&&!this.axisHelper&&(this.axisHelper=new zn(3),this.scene.add(this.axisHelper)),!this.options.toggleAxis&&this.axisHelper&&(this.axisHelper.removeFromParent(),this.axisHelper=null))}syncPostProcessing(){var t,a;if(!this.renderer||!this.scene||!this.camera)return;const e=this.container.clientWidth,n=this.container.clientHeight;if(this.currentOptions.grain){if(!this.composer||!this.grainPass){const s=new wn(this.renderer);s.setPixelRatio(Math.min(window.devicePixelRatio,this.currentOptions.pixelDensity)),s.setSize(e,n);const l=new En(this.scene,this.camera),f=new bn(Ft);f.enabled=!0,f.uniforms.width.value=e,f.uniforms.height.value=n,f.uniforms.blending.value=this.currentOptions.grainBlending,f.uniforms.disable.value=!1,s.addPass(l),s.addPass(f),this.composer=s,this.grainPass=f}this.grainPass.uniforms.width.value=e,this.grainPass.uniforms.height.value=n,this.grainPass.uniforms.blending.value=this.currentOptions.grainBlending,this.grainPass.uniforms.disable.value=!1;return}(t=this.grainPass)==null||t.dispose(),this.grainPass=null,(a=this.composer)==null||a.dispose(),this.composer=null}syncClock(){if(this.clock){if(this.currentOptions.animate){this.clock.running||this.clock.start();return}this.clock.running&&this.clock.stop()}}syncCameraControls(e){if(!(!this.camera||!this.renderer)){if(!this.cameraControls){const n=new W(this.camera,this.renderer.domElement);n.addEventListener("rest",()=>this.emitCameraUpdate()),n.mouseButtons.left=W.ACTION.ROTATE,n.mouseButtons.right=W.ACTION.NONE,n.touches.one=W.ACTION.ROTATE,n.touches.two=W.ACTION.NONE,n.touches.three=W.ACTION.NONE,this.cameraControls=n}if(this.cameraControls.enabled=this.options.enableCameraControls||this.options.enableCameraUpdate,this.cameraControls.smoothTime=e?Math.max(.05,this.options.smoothTime):0,this.cameraControls.dollySpeed=5,this.cameraControls.maxDistance=1e3,this.cameraControls.restThreshold=.01,this.cameraControls.mouseButtons.middle=this.options.type==="sphere"?W.ACTION.ZOOM:W.ACTION.DOLLY,this.cameraControls.mouseButtons.wheel=this.options.type==="sphere"?W.ACTION.ZOOM:W.ACTION.DOLLY,this.cameraControls.rotateTo(Ce(this.options.cAzimuthAngle),Ce(this.options.cPolarAngle),e),this.options.zoomOut){this.options.type==="sphere"?(this.cameraControls.dollyTo(ln.distance,e),this.cameraControls.zoomTo(ln.zoom,e)):(this.cameraControls.dollyTo(sn.distance,e),this.cameraControls.zoomTo(sn.zoom,e));return}if(this.options.type==="sphere"){this.cameraControls.zoomTo(this.options.cameraZoom,e),this.cameraControls.dollyTo(Ht,e);return}this.cameraControls.dollyTo(this.options.cDistance,e),this.cameraControls.zoomTo(Mt,e)}}emitCameraUpdate(){if(!this.cameraControls||!this.camera||!this.options.onCameraUpdate||!this.options.enableCameraUpdate)return;const e=Math.round(on(this.cameraControls.azimuthAngle)),n=Math.round(on(this.cameraControls.polarAngle));this.options.onCameraUpdate({cAzimuthAngle:e,cPolarAngle:n,cDistance:this.options.type==="sphere"?this.currentOptions.cDistance:Number(this.cameraControls.distance.toFixed(2)),cameraZoom:this.options.type==="sphere"?Number(this.camera.zoom.toFixed(2)):this.currentOptions.cameraZoom})}getAnimatedTime(){if(!this.clock)return this.currentOptions.uTime;if(!this.currentOptions.animate)return this.currentOptions.uTime;let e=this.clock.getElapsedTime();if(this.currentOptions.loop&&Number.isFinite(this.currentOptions.loopDuration)&&this.currentOptions.loopDuration>0)return e%this.currentOptions.loopDuration;if(!this.currentOptions.range)return e;const n=this.currentOptions.rangeStart,t=this.currentOptions.rangeEnd;if(!(Number.isFinite(n)&&Number.isFinite(t)&&t>n))return e;const a=n+e;return a>=t?(this.clock.start(),n):a}handleResize(){var t;const e=this.container.clientWidth,n=this.container.clientHeight;if(!(!e||!n)){if(!this.renderer){this.initScene();return}this.renderer.setSize(e,n),(t=this.composer)==null||t.setSize(e,n),this.grainPass&&(this.grainPass.uniforms.width.value=e,this.grainPass.uniforms.height.value=n),this.camera&&(this.camera.aspect=e/n,this.camera.updateProjectionMatrix())}}};const mn=j.createContext(null);function Xt(o,e,n){const t=j.useRef(null),[a,s]=j.useState(!o);return j.useEffect(()=>{if(!o||!t.current)return;const l=new IntersectionObserver(([f])=>{f!=null&&f.isIntersecting&&(s(!0),l.disconnect())},{threshold:e,rootMargin:n});return l.observe(t.current),()=>l.disconnect()},[o,e,n]),[t,a]}function Jt({children:o,style:e,className:n,pixelDensity:t=1,fov:a=45,pointerEvents:s="auto",lazyLoad:l=!0,threshold:f=.1,rootMargin:m="0px",preserveDrawingBuffer:p,powerPreference:P}){const[h,w]=Xt(l,f,m),[_,L]=j.useState(null);j.useEffect(()=>{L(h.current)},[h,w]);const E=j.useMemo(()=>({container:_,defaults:{pixelDensity:t,fov:a,preserveDrawingBuffer:p,powerPreference:P}}),[_,a,t,P,p]);return Xe.jsx("div",{ref:h,className:n,style:{position:"relative",width:"100%",height:"100%",pointerEvents:s,...e},children:(!l||w)&&Xe.jsx(mn.Provider,{value:E,children:o})})}function ei(o){const e=j.useContext(mn),n=j.useRef(null);if(!e)throw new Error("ShaderGradient must be used inside ShaderGradientCanvas.");const t={...e.defaults,...o};return j.useEffect(()=>{if(!e.container)return;const a=new Wt(e.container,t);return n.current=a,()=>{a.dispose(),n.current=null}},[e.container]),j.useEffect(()=>{n.current&&n.current.update(t)}),null}export{Hn as R,Jt as S,j as a,Kt as b,ei as c,Xe as j,fn as r};
