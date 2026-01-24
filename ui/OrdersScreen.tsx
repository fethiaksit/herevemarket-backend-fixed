import React from "react";
import { FlatList, SafeAreaView, StatusBar, StyleSheet, Text, View } from "react-native";

export type OrderStatus = "pending" | "preparing" | "delivered";

export interface OrderItem {
  id: string;
  number: string;
  status: OrderStatus;
  totalPrice: number;
  placedAt: string; // ISO string
}

const STATUS_STYLES: Record<OrderStatus, { label: string; backgroundColor: string; color: string }> = {
  pending: { label: "Pending", backgroundColor: "#FFF4E5", color: "#C26C02" },
  preparing: { label: "Preparing", backgroundColor: "#E8F3FF", color: "#1B6DD1" },
  delivered: { label: "Delivered", backgroundColor: "#E8FBF1", color: "#1F8A4D" },
};

interface OrderCardProps {
  order: OrderItem;
}

export const OrderCard: React.FC<OrderCardProps> = ({ order }) => {
  const statusStyle = STATUS_STYLES[order.status];
  const date = new Date(order.placedAt);
  const dateLabel = date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
  const timeLabel = date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });

  return (
    <View style={styles.card}>
      <View style={styles.cardHeader}>
        <Text style={styles.orderNumber}>{order.number}</Text>
        <View style={[styles.badge, { backgroundColor: statusStyle.backgroundColor }]}>
          <Text style={[styles.badgeText, { color: statusStyle.color }]}>{statusStyle.label}</Text>
        </View>
      </View>

      <View style={styles.row}>
        <Text style={styles.label}>Toplam</Text>
        <Text style={styles.value}>₺{order.totalPrice.toFixed(2)}</Text>
      </View>

      <View style={styles.row}>
        <Text style={styles.label}>Tarih</Text>
        <Text style={styles.value}>
          {dateLabel} · {timeLabel}
        </Text>
      </View>
    </View>
  );
};

const sampleOrders: OrderItem[] = [
  { id: "1", number: "#102394", status: "pending", totalPrice: 249.9, placedAt: new Date().toISOString() },
  { id: "2", number: "#102395", status: "preparing", totalPrice: 389.5, placedAt: new Date(Date.now() - 3600 * 1000).toISOString() },
  { id: "3", number: "#102396", status: "delivered", totalPrice: 159.0, placedAt: new Date(Date.now() - 3600 * 1000 * 24).toISOString() },
];

export const OrdersScreen: React.FC = () => {
  const data = sampleOrders;

  return (
    <SafeAreaView style={styles.container}>
      <StatusBar barStyle="dark-content" />
      <View style={styles.header}>
        <Text style={styles.title}>Siparişler</Text>
        <Text style={styles.subtitle}>Yeni ve geçmiş siparişlerin</Text>
      </View>

      {data.length === 0 ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyTitle}>Henüz sipariş yok</Text>
          <Text style={styles.emptySubtitle}>Siparişlerin burada listelenecek.</Text>
        </View>
      ) : (
        <FlatList
          data={data}
          keyExtractor={(item) => item.id}
          renderItem={({ item }) => <OrderCard order={item} />}
          contentContainerStyle={styles.listContent}
          ItemSeparatorComponent={() => <View style={styles.separator} />}
          showsVerticalScrollIndicator={false}
        />
      )}
    </SafeAreaView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#F7F8FA",
  },
  header: {
    paddingHorizontal: 20,
    paddingTop: 12,
    paddingBottom: 8,
  },
  title: {
    fontSize: 24,
    fontWeight: "700",
    color: "#0F172A",
  },
  subtitle: {
    marginTop: 4,
    fontSize: 14,
    color: "#475569",
  },
  listContent: {
    paddingHorizontal: 20,
    paddingBottom: 24,
  },
  separator: {
    height: 12,
  },
  card: {
    backgroundColor: "#FFFFFF",
    borderRadius: 16,
    padding: 16,
    shadowColor: "#000",
    shadowOpacity: 0.08,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 6 },
    elevation: 4,
  },
  cardHeader: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginBottom: 12,
  },
  orderNumber: {
    fontSize: 18,
    fontWeight: "700",
    color: "#0F172A",
  },
  badge: {
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 999,
  },
  badgeText: {
    fontSize: 12,
    fontWeight: "600",
    letterSpacing: 0.2,
  },
  row: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    marginTop: 6,
  },
  label: {
    fontSize: 14,
    color: "#64748B",
  },
  value: {
    fontSize: 15,
    fontWeight: "700",
    color: "#0F172A",
  },
  emptyState: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 24,
  },
  emptyTitle: {
    fontSize: 18,
    fontWeight: "700",
    color: "#0F172A",
  },
  emptySubtitle: {
    marginTop: 6,
    fontSize: 14,
    color: "#475569",
    textAlign: "center",
  },
});

export default OrdersScreen;
