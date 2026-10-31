# Linguagem ubíqua

Vocabulário usado no Event Storming, na documentação, na API e no código. O [modelo de domínio](dominio.md) detalha contextos, agregados e invariantes.

## Termos

| Termo | Significado no MVP |
|---|---|
| Cliente | Pessoa física ou jurídica responsável pelo veículo e pela decisão sobre o orçamento. |
| Funcionário | Pessoa autenticada que realiza as operações administrativas da oficina. Há um único perfil administrativo. |
| Veículo | Veículo identificado por placa única e vinculado a um cliente. |
| Serviço do catálogo | Definição reutilizável de um serviço, com nome e preço atual. |
| Item de serviço da OS | Uma execução de um serviço do catálogo naquela OS; tem identidade, estado e horários próprios. Não possui quantidade. |
| Peça / insumo | Item de estoque classificado como `PART` ou `SUPPLY`, movimentado em unidades inteiras. |
| Ordem de serviço (OS) | Registro do atendimento, do problema relatado, dos itens solicitados e de seu andamento. |
| Diagnóstico | Etapa na qual o funcionário define ou revisa serviços e peças necessários. |
| Orçamento | Versão dos serviços e peças, com nomes, preços, quantidades e total congelados para decisão do cliente. |
| Orçamento vigente | Última versão da OS que pode orientar a decisão ou a execução; uma versão superada não volta a produzir efeitos. |
| Aprovação | Aceite do orçamento vigente pelo cliente. Não inicia a execução nem reserva estoque. |
| Rejeição | Recusa do orçamento vigente; devolve a OS ao diagnóstico. Não cancela a OS. |
| Reabertura do diagnóstico | Ação anterior à execução que supera o orçamento pendente ou aprovado e exige novo orçamento e nova aprovação. |
| Baixa | Saída de todas as peças aprovadas, uma única vez, ao iniciar o primeiro serviço. |
| Movimento de estoque | Registro explícito de entrada ou saída com quantidade assinada e motivo; pode referenciar a OS. |
| Finalização | Encerramento técnico da OS após concluir todos os seus serviços. |
| Entrega | Registro de devolução do veículo, posterior à finalização. |
| Token de acesso à OS | Credencial limitada a uma OS, enviada como Bearer na API e com validade; não é uma busca pública por documento ou placa. |
| Tempo de execução | Intervalo entre início e conclusão de um serviço, ou entre início da execução e finalização da OS; exclui diagnóstico e espera por aprovação. |

## Estados da OS no código

| Termo | Valor | Constante em `backend/internal/domain/order.go` |
|---|---|---|
| Recebida | `RECEIVED` | `domain.Received` |
| Em diagnóstico | `IN_DIAGNOSIS` | `domain.Diagnosis` |
| Aguardando aprovação | `AWAITING_APPROVAL` | `domain.Awaiting` |
| Em execução | `IN_EXECUTION` | `domain.Executing` |
| Finalizada | `COMPLETED` | `domain.Completed` |
| Entregue | `DELIVERED` | `domain.Delivered` |

Orçamento: `PENDING`, `APPROVED`, `REJECTED` e `SUPERSEDED` (superado pela reabertura). Item de serviço: `NOT_STARTED`, `IN_PROGRESS` e `COMPLETED`.

## Eventos do Event Storming no histórico da OS

| Evento no board | Registro no histórico | Origem |
|---|---|---|
| OS aberta | `ORDER_RECEIVED` | `workorders.Service.Create` |
| Diagnóstico iniciado | `DIAGNOSIS_STARTED` | `Order.StartDiagnosis` |
| Diagnóstico reaberto | `DIAGNOSIS_REOPENED` | `Order.Reopen` |
| Orçamento publicado | `QUOTE_PUBLISHED` | `Order.Publish` |
| Orçamento aprovado / rejeitado | `QUOTE_APPROVED` / `QUOTE_REJECTED` | `Order.Decide` |
| Serviço iniciado | `SERVICE_STARTED` | `Order.StartService` |
| Peças baixadas do estoque | movimento `SERVICE_ORDER_CONSUMPTION` | `workorders` no primeiro início de serviço |
| Serviço concluído | `SERVICE_COMPLETED` | `Order.CompleteService` |
| OS finalizada | `ORDER_COMPLETED` | `Order.Complete` |
| Veículo entregue | `ORDER_DELIVERED` | `Order.Deliver` |

## Fora do MVP

Termos que surgiram no brainstorm do Event Storming e ficaram para as próximas fases: pagamento (sinal e saldo), compras a fornecedores, prazo de chegada de peças, agendamento, gestão de pátio, envio do orçamento por e-mail ou WhatsApp e aceite presencial do orçamento.
