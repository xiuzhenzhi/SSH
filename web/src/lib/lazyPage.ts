import { defineAsyncComponent, type AsyncComponentLoader, type Component } from 'vue';
import PageLoading from '../components/PageLoading.vue';

export function lazyPage<T extends Component>(loader: AsyncComponentLoader<T>) {
  return defineAsyncComponent({loader,loadingComponent:PageLoading,errorComponent:PageLoading,delay:120,timeout:10000});
}
