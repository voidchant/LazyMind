import {act,renderHook,waitFor} from '@testing-library/react';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {usePaperArticle} from './paperArticle';
const api=vi.hoisted(()=>({get:vi.fn(),post:vi.fn()}));
vi.mock('@/components/request',()=>({axiosInstance:api,BASE_URL:''}));
vi.mock('./paperReader',()=>({usePaperReaderSession:()=> 'user'}));
const article={schema_version:1,source_sha256:'source',title:'A method',dek:'Input to output',sections:[],assets:[],evidence:[],warnings:[],metrics:{generation_id:'g1',model:'model',duration_ms:1000,llm_calls:1,cost_status:'unavailable',started_at:''}};
const target={dataset:'d',document:'p'};
beforeEach(()=>{api.get.mockReset();api.post.mockReset();});
afterEach(()=>vi.useRealTimers());
it('reads existing results without starting generation, including reopen',async()=>{
 api.get.mockResolvedValue({data:{data:{status:'ready',article}}});
 const view=renderHook(({active})=>usePaperArticle(target,'source','en',active),{initialProps:{active:true}});
 await waitFor(()=>expect(view.result.current.article?.title).toBe('A method'));
 view.rerender({active:false});view.rerender({active:true});
 await waitFor(()=>expect(api.get).toHaveBeenCalledTimes(2));expect(api.post).not.toHaveBeenCalled();
 expect(api.get.mock.calls[0][1].params).toEqual({language:'en',source_sha256:'source'});
});
it('starts only missing articles and preserves a valid article during failed regeneration',async()=>{
 api.get.mockResolvedValue({data:{data:{status:'not_requested'}}});
 api.post.mockResolvedValueOnce({data:{data:{status:'ready',article}}}).mockRejectedValue(new Error('offline'));
 const {result}=renderHook(()=>usePaperArticle(target,'source','en',true));
 await waitFor(()=>expect(result.current.status).toBe('ready'));
 expect(api.post).toHaveBeenCalledTimes(1);
 await act(async()=>{await result.current.generate(true);});
 expect(result.current.status).toBe('failed');expect(result.current.article?.title).toBe('A method');
 expect(api.post.mock.calls[1][1].force).toBe(true);
});
it('does not expose a response for a different PDF hash',async()=>{
 api.get.mockResolvedValue({data:{data:{status:'ready',article:{...article,source_sha256:'wrong'}}}});
 const {result}=renderHook(()=>usePaperArticle(target,'source','en',true));
 await waitFor(()=>expect(result.current.status).toBe('failed'));
 expect(result.current.article).toBeUndefined();expect(result.current.sourceMismatch).toBe(true);
});
it('drops the previous article immediately when the source changes',async()=>{
 api.get.mockResolvedValueOnce({data:{data:{status:'ready',article}}}).mockReturnValue(new Promise(()=>{}));
 const view=renderHook(({hash})=>usePaperArticle(target,hash,'en',true),{initialProps:{hash:'source'}});
 await waitFor(()=>expect(view.result.current.article).toBeDefined());view.rerender({hash:'new'});
 expect(view.result.current.article).toBeUndefined();
});
