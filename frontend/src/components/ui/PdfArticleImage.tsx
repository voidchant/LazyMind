import {useEffect,useRef,useState} from 'react';
import {Modal} from 'antd';
import {axiosInstance,BASE_URL} from '@/components/request';
import {usePaperReaderSession} from '@/modules/knowledge/api/paperReader';
import type {PdfIRBlock,PdfIRDocument,PdfIRTarget} from '@/modules/knowledge/api/pdfIR';
interface Props {target:PdfIRTarget;document:PdfIRDocument;block:PdfIRBlock;caption:string;language:string;onUnavailable?:(id:string,failed:boolean)=>void}
export function PdfArticleImage({target,document,block,caption,language,onUnavailable}:Props) {
 const identity=usePaperReaderSession();
 const key=`${identity}/${target.dataset}/${target.document}/${document.source_sha256}/${block.id}`;
 const [state,setState]=useState<{key:string;url?:string;error?:boolean}>();
 const [attempt,setAttempt]=useState(0),[expanded,setExpanded]=useState(false);
 const notify=useRef(onUnavailable);notify.current=onUnavailable;
 const zh=language==='zh';
 useEffect(()=>{
  const controller=new AbortController();let objectURL:string|undefined;
  setState({key});setExpanded(false);notify.current?.(block.id,false);
  const load=async()=>{
   try {
    const url=`${BASE_URL}/api/core/datasets/${encodeURIComponent(target.dataset)}/documents/${encodeURIComponent(target.document)}/pdf-ir/blocks/${encodeURIComponent(block.id)}/image`;
    const response=await axiosInstance.get<Blob>(url,{params:{source_sha256:document.source_sha256},signal:controller.signal,responseType:'blob',timeout:30000,silentError:true} as never);
    if(controller.signal.aborted)return;
    if(!(response.data instanceof Blob)||!/^image\/(png|jpeg|webp)$/i.test(response.data.type))throw new Error('Unsupported image');
    objectURL=URL.createObjectURL(response.data);setState({key,url:objectURL});
   }catch {if(!controller.signal.aborted){setState({key,error:true});notify.current?.(block.id,true);}}
  };
  void load();
  return()=>{controller.abort();if(objectURL)URL.revokeObjectURL(objectURL);notify.current?.(block.id,false);};
 },[key,attempt]);
 const current=state?.key===key?state:undefined;
 const page=document.pages.find(page=>page.index===block.page);
 const ratio=block.bbox?((block.bbox[2]-block.bbox[0])/(block.bbox[3]-block.bbox[1]))*((page?.width||1)/(page?.height||1)):undefined;
 const aspectRatio=ratio&&Number.isFinite(ratio)?Math.max(.3,Math.min(5,ratio)):undefined;
 return <>
  <div className="pdf-article-image" style={!current?.url&&aspectRatio?{aspectRatio}:undefined}>
   {current?.url?<button type="button" className="pdf-article-image-zoom" onClick={()=>setExpanded(true)} aria-label={zh?'放大原图':'Enlarge original figure'}><img src={current.url} alt={caption||`${zh?'原论文图片':'Original paper figure'} · ${block.page+1}`} onError={()=>{setState({key,error:true});notify.current?.(block.id,true);}}/><span>{zh?'点击放大':'Click to enlarge'}</span></button>
    :current?.error?<div role="status"><p>{zh?'原图暂时无法加载，仍可查看原文位置。':'The original figure could not load. Its source location remains available.'}</p><button onClick={()=>setAttempt(value=>value+1)}>{zh?'重试图片':'Retry image'}</button></div>
    :<div role="status">{zh?'正在加载原论文图片…':'Loading original figure…'}</div>}
  </div>
  <Modal open={expanded&&!!current?.url} onCancel={()=>setExpanded(false)} footer={null} width="min(1200px, 96vw)" title={zh?'原图放大':'Original figure'} destroyOnClose>
   <div className="pdf-article-image-lightbox"><img src={current?.url} alt={caption}/></div>
   {caption&&<p className="pdf-article-lightbox-caption">{caption}</p>}
  </Modal>
 </>;
}
