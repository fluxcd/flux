Akı

GitOps'a inanıyoruz:

Sisteminizin istediğiniz halini deklare olarak tanımlayın. Buna uygulamalar, yapılandırma, kontrol panelleri, izleme ve diğer her şey dahildir.

Açıklanabilir ne otomatik olabilir. Sistemin uyumluluğunu sağlamak için YAML'leri kullanın. Kubectl'i çalıştırmanıza gerek yok, tüm değişiklikler git geçiyor. Gözlenen ve istenen durum arasındaki ayrımı tespit etmek ve bildirimleri almak için farklı araçlar kullanın.

Kodları konteynere değil. Her şey çekme istekleri ile kontrol edilir. Yeni dev'ler için öğrenme eğrisi yoktur, sadece standart git PR işleminizi kullanırlar. Git'teki geçmiş, bir dizi işleminiz olduğu için herhangi bir anlık görüntüden kurtarmanızı sağlar. Çekme talebi ile operasyonel değişiklikler yapmak çok daha şeffaftır, örn. Çalışan sistemde değişiklik yapmak yerine bir çekme talebi ile bir üretim sorunu düzeltin.

Akı, bir kümenin durumunun <a1> Git </ a1> yapılandırma ile eşleşmesini otomatik olarak sağlayan bir araçtır. Kubernetes içindeki dağıtımları tetiklemek için kümedeki bir operatör kullanır, bu da ayrı bir CD aracına ihtiyacınız olmadığı anlamına gelir. İlgili tüm görüntü depolarını izler, yeni görüntüleri algılar, dağıtımları tetikler ve buna bağlı olarak çalışan yapılandırmayı günceller (ve yapılandırılabilir bir ilke).

Avantajlar şunlardır: CI'nize küme erişiminizi vermeniz gerekmez, her değişiklik atomik ve işlemseldir, git denetim günlüğünüze sahiptir. Her işlem ya başarısız oluyor ya da başarılı bir şekilde başarılı oluyor. Tamamen kod merkezli ve yeni altyapıya ihtiyacınız yok.

￼

￼ ￼

Ne akı yapar

Akı, Sürekli Dağıtım hattının sonunda bir dağıtım aracı olarak kullanıldığında en faydalıdır. Akı, yeni kap resimlerinizin ve yapılandırma değişikliklerinin kümeye yayıldığından emin olur.

Özellikler

Başlıca özellikleri:

Otomatikleştirilmiş git → küme senkronizasyonu

Yeni konteyner görsellerinin otomatik olarak dağıtımı

Diğer devops araçlarla entegrasyonlar (Helm ve daha fazlası)

Ek bir hizmet veya altyapı gerektirmez - Akı kümenizin içinde yaşar

Kümedeki dağıtımların durumuyla ilgili ileri düzey kontrol (geri alma, bir iş yükünün belirli bir sürümünü kilitleme, manuel dağıtımlar)

Gözlemlenebilirlik: git taahhütleri bir denetleme izidir ve örneğin belirli bir dağıtımın neden kilitlendiğini kaydedebilirsiniz.

Örgü Bulut İlişkisi

Weave Cloud, Weaveworks tarafından Flux içeren bir SaaS ürünüdür.

Bir UI ve dağıtımlar için uyarılar: güzel entegre genel bakış, tüm Flux işlemleri sadece bir tık uzakta.

Kümeniz için tam gözlenebilirlik ve içgörüler: Kümeniz için izleme panellerini kullanmaya başlayın, 13 aylık bir geçmişe ev sahipliği yapın, kendi kümenizin gerçek zamanlı haritasını kullanarak durumunun hatalarını ayıklayın ve analiz edin.

Weave Cloud hakkında daha fazla bilgi edinmek isterseniz, onu ana sayfasında görüyorsunuz.

Flux ile başlayın

Aşağıdaki dokümanlara göz atarak işe başlayın:

Akı hakkında bilgi

Akıya Giriş

SSS ve sık karşılaşılan sorunlar

Nasıl çalışır

Flux kurulumu ile ilgili hususlar

Akı <-> Dümen entegrasyonu

Akı ile Başlayın

Bağımsız Akı

Helm kullanarak akı

Çalışma akı

Akı Kullanımı

Helm Operatörü

Sorun giderme

Sık karşılaşılan sorunlar

Flux v1'e Yükseltme

Topluluk ve Geliştirici bilgileri

Flux'e her türlü katkıyı memnuniyetle karşılıyoruz, kodlar, bulduğunuz konular, belgeler, harici araçlar, yardım ve destek ya da başka bir şey olsun.

Küfürlü, taciz edici veya başka türlü kabul edilemez davranışların örnekleri Flux proje denetçisi veya Alexis Richardson <alexis@weave.works> ile iletişime geçerek bildirilebilir. Lütfen davranış kurallarımıza da bakınız.

Kendinizi projeye ve işlerin nasıl yürüdüğüne alıştırmak için aşağıdakileri ilginizi çekebilir:

Katkılarımızla ilgili yönergeler

Belge oluştur

Sürüm belgeleri

Yardım almak

Flux ve sürekli teslimat hakkında herhangi bir sorunuz varsa:

Dokuma Akışı dokümanlarını okuyun.

Kendinizi Örgü topluluk gevşekliğine davet edin.

#Flux gevşek kanalında bir soru sorun.

Weave User Group'a katılın ve bölgenizdeki çevrimiçi görüşmelere, uygulamalı eğitime ve buluşmalara davet edin.

Weave-users@weave.works adresine bir email gönder

Bir sorun var.

Görüşleriniz her zaman açığız!
