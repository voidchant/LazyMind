import {useCallback,useEffect,useRef,useState} from 'react';
import {axiosInstance,BASE_URL} from '@/components/request';
import {usePaperReaderSession} from './paperReader';
import type {PdfIRBlock,PdfIRTarget} from './pdfIR';
import type {FormulaEvidence} from './formulaExplanation';

export interface ArticleBlock {
 kind:'paragraph'|'figure'|'table'|'formula';
 text:string;
 source_ids:string[];
 attribution:'author'|'reported'|'analysis'|'uncertain';
 asset_id?:string;
 caption?:string;
}
export interface ArticleSection {id:string;title:string;role:string;blocks:ArticleBlock[]}
export interface ArticleMetrics {
 generation_id:string;model:string;started_at:string;duration_ms:number;llm_calls:number;
 input_tokens?:number;output_tokens?:number;cost_usd?:number;cost_status:string;cache_hit?:boolean;
}
export interface PaperArticle {
 schema_version:1;source_sha256:string;title:string;dek:string;sections:ArticleSection[];
 assets:PdfIRBlock[];evidence:FormulaEvidence[];warnings:string[];metrics:ArticleMetrics;
}
export interface ArticleState {
 status:'not_requested'|'generating'|'ready'|'partial'|'failed';
 article?:PaperArticle;error_message?:string;
}
interface ClientState extends Omit<ArticleState,'status'> {key:string;status:ArticleState['status']|'checking';sourceMismatch?:boolean}
export function articleURL(target:PdfIRTarget) {
 return `${BASE_URL}/api/core/datasets/${encodeURIComponent(target.dataset)}/documents/${encodeURIComponent(target.document)}/pdf-ir/article`;
}
/** Server state is authoritative: reopening reads the existing job/result rather than paying again. */
export function usePaperArticle(target:PdfIRTarget,hash:string,language:string,active:boolean) {
 const identity=usePaperReaderSession();
 const key=`${identity}/${target.dataset}/${target.document}/${hash}/${language}`;
 const [state,setState]=useState<ClientState>();
 const request=useRef<AbortController>();
 const timer=useRef<ReturnType<typeof setTimeout>>();
 const currentKey=useRef(key);currentKey.current=key;
 const isActive=useRef(active);isActive.current=active;
 const cancel=useCallback(()=>{request.current?.abort();clearTimeout(timer.current);},[]);
 const run=useCallback(async(method:'get'|'post',force=false)=>{
  cancel();
  const controller=new AbortController();request.current=controller;
  setState(previous=>({key,status:method==='post'?'generating':'checking',article:previous?.key===key?previous.article:undefined}));
  const valid=()=>!controller.signal.aborted&&currentKey.current===key;
  try {
   const params={language,source_sha256:hash};
   const options={signal:controller.signal,timeout:30000,silentError:true};
   let response=method==='get'
    ?await axiosInstance.get<{data:ArticleState}>(articleURL(target),{...options,params} as never)
    :await axiosInstance.post<{data:ArticleState}>(articleURL(target),{...params,force},options as never);
   if(!valid())return;
   if(method==='get'&&response.data.data.status==='not_requested'){
    setState({key,status:'generating'});
    response=await axiosInstance.post<{data:ArticleState}>(articleURL(target),{...params,force:false},options as never);
    if(!valid())return;
   }
   const next=response.data.data;
   if(next.article&&(next.article.source_sha256!==hash||next.article.schema_version!==1)){
    setState({key,status:'failed',sourceMismatch:true});return;
   }
   setState(previous=>({...next,key,article:next.article||(previous?.key===key?previous.article:undefined)}));
   if(next.status==='generating'&&isActive.current)timer.current=setTimeout(()=>void run('get'),1500);
  }catch(error){
   if(!valid())return;
   const response=(error as {response?:{status?:number;data?:{data?:{error_message?:string};message?:string}}}).response;
   const sourceMismatch=response?.status===409;
   const forbidden=response?.status===401||response?.status===403||response?.status===404;
   setState(previous=>({key,status:'failed',sourceMismatch,error_message:response?.data?.data?.error_message||response?.data?.message,article:!sourceMismatch&&!forbidden&&previous?.key===key?previous.article:undefined}));
  }
 },[cancel,key,target.dataset,target.document,hash,language]);
 useEffect(()=>{if(active)void run('get');return cancel;},[active,run,cancel]);
 const generate=useCallback((force=false)=>run('post',force),[run]);
 return {...(state?.key===key?state:{status:'checking' as const}),generate};
}
