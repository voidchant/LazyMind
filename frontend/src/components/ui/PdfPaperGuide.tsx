import {useCallback,useEffect,useMemo,useState} from 'react';
import type {PdfIRBlock,PdfIRDocument,PdfIRTarget} from '@/modules/knowledge/api/pdfIR';
import type {FormulaEvidence} from '@/modules/knowledge/api/formulaExplanation';
import {readablePaperText} from '@/modules/knowledge/api/paperReader';
import {usePaperArticle,type ArticleBlock,type ArticleMetrics} from '@/modules/knowledge/api/paperArticle';
import type {PdfTextSelection} from './RenderPdf';
import {FormulaMathText} from './FormulaMathText';
import {usePaperReaderLabels,paperVisualLabel} from './paperReaderLabels';
import {PdfArticleImage} from './PdfArticleImage';
import './PdfPaperOverview.css';
interface Props{target:PdfIRTarget;document:PdfIRDocument;active:boolean;paperTitle?:string;onNavigate:(id:string)=>void;onStudy:(block:PdfIRBlock)=>void;onAsk?:(selection:PdfTextSelection)=>void}
// Only explicit title blocks and explicitly labelled abstracts qualify as metadata.
function paperMetadata(document:PdfIRDocument,paperTitle?:string){
 const firstPage=document.blocks.filter(block=>block.page===0);
 const isAbstractLabel=(block:PdfIRBlock)=>/^(?:abstract|摘要)\s*[.:：]?$/i.test(block.text?.trim()||'');
 const title=firstPage.find(block=>block.type==='title'&&block.text?.trim()&&!isAbstractLabel(block));
 const abstract=firstPage.find(block=>block.type==='abstract'&&!isAbstractLabel(block)||/^(?:abstract|摘要)\s*[.:：—-]\s*\S/i.test(block.text?.trim()||''));
 let abstractText=abstract?.text?.replace(/^(?:abstract|摘要)\s*[.:：—-]?\s*/i,'')||'';
 if(!abstractText){
  const headingIndex=firstPage.findIndex(isAbstractLabel);
  if(headingIndex>=0){
   const paragraphs:string[]=[];
   for(const block of firstPage.slice(headingIndex+1)){
    if(block.type==='heading'||block.type==='title'||/^(?:\d+(?:\.\d+)*\.?\s+)?(?:introduction|background|keywords|引言|绪论|关键词)\s*[.:：]?$/i.test(block.text?.trim()||''))break;
    if(block.type!=='text'||!block.text?.trim())continue;
    if(paragraphs.length>=3||paragraphs.join(' ').length+block.text.length>6000)break;
    paragraphs.push(block.text.trim());
   }
   abstractText=paragraphs.join(' ');
  }
 }
 return {title:readablePaperText(title?.text||paperTitle||''),abstract:readablePaperText(abstractText)};
}
function ArticleTable({block}:{block:PdfIRBlock}) {
 if(!block.table)return null;
 return <>
  <div className="pdf-overview-table" tabIndex={0} role="region" aria-label={block.table.captions.join(' ')||'Source table'}><table>
   {block.table.captions.length>0&&<caption><FormulaMathText text={block.table.captions.join('\n')}/></caption>}
   <tbody>{block.table.rows.map((row,index)=><tr key={index}>{row.cells.map((cell,column)=>{
    const Cell=cell.header?'th':'td';
    return <Cell key={column} rowSpan={cell.row_span||1} colSpan={cell.col_span||1}><FormulaMathText text={cell.text}/></Cell>;
   })}</tr>)}</tbody>
  </table></div>
  {block.table.footnotes.map((text,index)=><p key={index} className="pdf-article-table-note"><FormulaMathText text={text}/></p>)}
 </>;
}
function GenerationMetrics({metrics,zh}:{metrics:ArticleMetrics;zh:boolean}) {
 const number=(value:number|undefined)=>typeof value==='number'&&Number.isFinite(value)?value.toLocaleString():zh?'未提供':'Unavailable';
 return <details className="pdf-article-metrics"><summary>{zh?'生成记录与使用量':'Generation & usage'}</summary><dl>
  <div><dt>{zh?'模型':'Model'}</dt><dd>{metrics.model}</dd></div>
  <div><dt>{zh?'生成时间':'Generated'}</dt><dd>{metrics.started_at?new Date(metrics.started_at).toLocaleString():number(undefined)}</dd></div>
  <div><dt>{zh?'耗时':'Duration'}</dt><dd>{Number.isFinite(metrics.duration_ms)?`${(metrics.duration_ms/1000).toFixed(1)}s`:number(undefined)}</dd></div>
  <div><dt>{zh?'实际模型调用':'Model calls'}</dt><dd>{number(metrics.llm_calls)}</dd></div>
  <div><dt>{zh?'输入 / 输出 token':'Input / output tokens'}</dt><dd>{number(metrics.input_tokens)} / {number(metrics.output_tokens)}</dd></div>
  <div><dt>{zh?'费用（USD）':'Cost (USD)'}</dt><dd>{typeof metrics.cost_usd==='number'?`$${metrics.cost_usd.toFixed(6)}`:zh?'供应方未提供可计费数据':'Billing data unavailable'}</dd></div>
  <div><dt>{zh?'结果复用':'Result reuse'}</dt><dd>{metrics.cache_hit?(zh?'本次读取已有结果':'Loaded existing result'):(zh?'生成记录已保存':'Generation record saved')}</dd></div>
 </dl></details>;
}
export function PdfPaperGuide({target,document,active,paperTitle,onNavigate,onStudy,onAsk}:Props) {
 const {label,language}=usePaperReaderLabels();const zh=language==='zh';
 const request=usePaperArticle(target,document.source_sha256,language,active);
 const article=request.article;
 const busy=request.status==='checking'||request.status==='generating';
 const sourceBlocks=useMemo(()=>new Map(document.blocks.map(block=>[block.id,block])),[document]);
 const assets=new Map((article?.assets||[]).filter(block=>sourceBlocks.get(block.id)?.type===block.type).map(block=>[block.id,block]));
 const evidence=new Map((article?.evidence||[]).filter(item=>sourceBlocks.has(item.id)).map(item=>[item.id,item]));
 const metadata=paperMetadata(document,paperTitle);
 const [unavailableImages,setUnavailableImages]=useState<string[]>([]);
 useEffect(()=>setUnavailableImages([]),[document.source_sha256,article?.metrics.generation_id]);
 const imageStatus=useCallback((id:string,failed:boolean)=>setUnavailableImages(current=>failed?(current.includes(id)?current:[...current,id]):current.filter(value=>value!==id)),[]);
 const attribution=(value:ArticleBlock['attribution'])=>({author:zh?'作者陈述':'Authors’ statement',reported:zh?'原文报告':'Reported result',analysis:zh?'教学解释 / 分析':'Explanation / analysis',uncertain:zh?'证据不足':'Evidence incomplete'}[value]|| (zh?'教学解释 / 分析':'Explanation / analysis'));
 const sourcesFor=(block:ArticleBlock)=>Array.from(new Set(block.source_ids||[])).map(id=>evidence.get(id)).filter((item):item is FormulaEvidence=>!!item);
 const ask=(block:ArticleBlock,sources:FormulaEvidence[])=>{
  if(!onAsk||!sources.length)return;
  const first=sources[0],source=sourceBlocks.get(first.id)!;
  const context=sources.map(item=>`[${item.id}; ${zh?'第':'p.'} ${sourceBlocks.get(item.id)!.page+1}${zh?' 页':''}] ${item.text}`).join('\n\n');
  onAsk({text:`${zh?'解读段落（模型生成）':'Article paragraph (generated)'}：${block.text}\n\n${zh?'原文依据':'Source excerpts'}：\n${context}`,page:source.page+1,bbox:source.bbox,context:`${metadata.title}\nsource_sha256: ${document.source_sha256}\n${context}`});
 };
 return <article className="pdf-paper-guide pdf-paper-overview">
  <header className="pdf-overview-paper-header">
   <div className="pdf-article-eyebrow">{zh?'方法导向 · 论文图文解读':'METHOD-FIRST · PAPER EXPLAINER'}</div>
   <h1>{article?.title||metadata.title||(zh?'读懂这篇论文的方法':'Understanding the method')}</h1>
   {article?.dek&&<p className="pdf-article-dek"><FormulaMathText text={article.dek}/></p>}
   {metadata.abstract&&<details className="pdf-overview-abstract"><summary>{zh?'展开原始 Abstract':'Original abstract'}</summary><p><FormulaMathText text={metadata.abstract}/></p></details>}
   <div className="pdf-article-toolbar"><span>{zh?'AI 解读；原文定位不代表语义已经验证。':'AI interpretation. Source links locate evidence; they do not verify its meaning.'}</span><button onClick={()=>void request.generate(!!article)} disabled={busy}>{request.status==='failed'?label('retry'):article?label('regenerate'):zh?'生成解读':'Generate article'}</button></div>
  </header>
  {busy&&<div className="pdf-reader-status" role="status">{request.status==='generating'?(zh?'正在组织方法、图表和原文依据；离开页面不会取消生成。':'Organizing the method, figures and evidence. Generation continues if you leave this page.'):(zh?'正在读取已有解读…':'Checking saved article…')}</div>}
  {request.status==='failed'&&<div className="pdf-reader-error" role="alert">{request.sourceMismatch?label('sourceChanged'):(request.error_message||label('error'))}{article&&<p>{zh?'保留上一次有效解读，你可以继续阅读。':'The previous valid article remains available.'}</p>}</div>}
  {(request.status==='partial'||unavailableImages.length>0)&&<p className="pdf-article-partial" role="status">{zh?'解读部分完成：下方会标出缺少依据或无法加载的素材，请结合原文核查。':'Partially complete: unavailable material or evidence gaps are marked below.'}</p>}
  {!article&&busy&&<div className="pdf-article-loading" aria-hidden="true"><span/><span/><span/></div>}
  {article&&<div className="pdf-overview-sections">{article.sections.map((section,sectionIndex)=><section key={section.id||sectionIndex} className="pdf-overview-section" id={`paper-article-section-${sectionIndex}`}>
   <h2>{section.title}</h2>
   {section.blocks.map((block,index)=>{
    const asset=block.asset_id?assets.get(block.asset_id):undefined;
    const sources=sourcesFor(block);
    const paragraphs=block.text.split(/\n\s*\n/).filter(Boolean);
    return <div className={`pdf-article-block is-${block.kind}`} key={`${section.id}-${index}`}>
     {block.kind==='formula'&&paragraphs[0]&&<p><FormulaMathText text={paragraphs[0]}/></p>}
     {asset&&block.kind!=='paragraph'&&<figure className={`pdf-article-visual is-${block.kind}`}>
      {block.kind==='figure'&&asset.type==='image'&&<PdfArticleImage target={target} document={document} block={asset} caption={block.caption||asset.image?.captions.join(' ')||asset.text||''} language={language} onUnavailable={imageStatus}/>}
      {block.kind==='table'&&asset.type==='table'&&<ArticleTable block={asset}/>}
      {block.kind==='formula'&&asset.type==='formula'&&<div className="pdf-overview-formula"><FormulaMathText text={asset.latex||asset.text||''} mathOnly/></div>}
      <figcaption>{block.caption&&<span><FormulaMathText text={block.caption}/></span>}<div><button onClick={()=>onNavigate(asset.id)}>{label('sourcePage',{page:sourceBlocks.get(asset.id)!.page+1})}</button><button onClick={()=>onStudy(sourceBlocks.get(asset.id)!)}>{block.kind==='formula'?(zh?'符号与公式精读':'Symbols & formula study'):paperVisualLabel(asset,document.blocks,label)}</button></div></figcaption>
     </figure>}
     {block.kind!=='paragraph'&&!asset&&<p className="pdf-article-asset-missing">{zh?'这处素材暂不可用，请查看所引用的原文。':'This source asset is unavailable; inspect the cited original.'}</p>}
     {(block.kind==='formula'?paragraphs.slice(1):paragraphs).map((text,part)=><p key={part}><FormulaMathText text={text}/></p>)}
     <div className="pdf-article-evidence"><span className={`pdf-article-attribution is-${block.attribution}`}>{attribution(block.attribution)}</span>{sources.map(item=><button key={item.id} title={item.text} onClick={()=>onNavigate(item.id)}>{label('sourcePage',{page:sourceBlocks.get(item.id)!.page+1})}</button>)}{onAsk&&sources.length>0&&<button className="pdf-article-ask" onClick={()=>ask(block,sources)}>{zh?'追问这一段':'Ask about this paragraph'}</button>}</div>
    </div>;
   })}
  </section>)}</div>}
  {article?.warnings.length? <details className="pdf-reader-notes"><summary>{zh?'阅读时需要留意':'Reading notes'}</summary>{article.warnings.map((warning,index)=><p key={index}>{warning}</p>)}</details>:null}
  {article?.metrics&&<GenerationMetrics metrics={article.metrics} zh={zh}/>}
 </article>;
}
