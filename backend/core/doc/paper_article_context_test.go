package doc

import (
 "strings"
 "testing"
 "lazymind/core/pdfir"
)

func TestArticleContextPrioritizesMethodAndCanonicalTables(t *testing.T) {
 d:=pdfir.Document{Blocks:[]pdfir.Block{{ID:"intro",Type:"heading",Text:"1 Introduction"}}}
 for i:=0;i<300;i++ {d.Blocks=append(d.Blocks,pdfir.Block{ID:strings.Repeat("x",i+1),Type:"text",Text:strings.Repeat("background ",150)})}
 d.Blocks=append(d.Blocks,pdfir.Block{ID:"method",Type:"heading",Text:"3 Method"},pdfir.Block{ID:"mechanism",Type:"text",Text:"Inputs pass through the shared encoder; only training uses paired supervision."},pdfir.Block{ID:"table",Type:"table",Text:"<table>bad extraction</table>",Table:&pdfir.Table{Captions:[]string{"Same backbone, same split"},Rows:[]pdfir.TableRow{{Cells:[]pdfir.TableCell{{Text:"Method"},{Text:"Accuracy (%)"}}},{Cells:[]pdfir.TableCell{{Text:"Baseline"},{Text:"81.7"}}}},Footnotes:[]string{"Mean over three seeds"}}})
 evidence,_,_:=paperArticleContext(d)
 all:="";for _,e:=range evidence {all+=e.Text}
 if !strings.Contains(all,"shared encoder")||!strings.Contains(all,"81.7")||!strings.Contains(all,"three seeds")||strings.Contains(all,"bad extraction") {t.Fatal("method or full canonical table context missing")}
}

func TestArticleValidationResolvesOnlyTrustedAssetsAndEvidence(t *testing.T) {
 ev:=[]formulaEvidence{{ID:"method",Text:"The output is an embedding."},{ID:"fig",Text:"Architecture"}}
 assets:=[]pdfir.Block{{ID:"fig",Type:"image",Page:3,BBox:[]float64{.1,.2,.8,.5}}}
 raw:=`{"title":"How it works","dek":"A method walkthrough","sections":[{"title":"Follow the embedding","role":"method","blocks":[{"kind":"paragraph","text":"An embedding is produced.","source_ids":["method"],"attribution":"author"},{"kind":"figure","text":"Follow the encoder.","asset_id":"fig","source_ids":["method","fig"],"attribution":"analysis","url":"https://evil.test","page":99},{"kind":"figure","text":"Fabricated visual","asset_id":"made-up","source_ids":["unknown"],"attribution":"reported"}]}]}`
 article,partial,err:=validatePaperArticle(raw,ev,assets,"zh")
 if err!=nil||!partial {t.Fatalf("expected partial safe article: %v",err)}
 if len(article.Assets)!=1||article.Assets[0].Page!=3 {t.Fatal("untrusted asset position survived")}
 if len(article.Sections[0].Blocks)!=2 {t.Fatal("unknown asset was retained")}
 if len(article.Evidence)!=2 {t.Fatal("server evidence missing")}
}

func TestArticleReportedNumbersBecomeUncertainWhenAbsentFromEvidence(t *testing.T) {
 raw:=`{"title":"Method","dek":"A guide","sections":[{"title":"Result","role":"evidence","blocks":[{"kind":"paragraph","text":"Accuracy was 99.9%.","source_ids":["a"],"attribution":"reported"}]}]}`
 a,partial,e:=validatePaperArticle(raw,[]formulaEvidence{{ID:"a",Text:"Accuracy was 81.7%."}},nil,"en")
 if e!=nil||!partial||a.Sections[0].Blocks[0].Attribution!="uncertain" {t.Fatal("unsupported reported number accepted")}
}
