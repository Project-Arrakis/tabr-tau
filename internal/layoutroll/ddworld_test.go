package layoutroll

// A made-up world, the shape of a real base in miniature (the same rows as the base export tests of internal/ops): the totem 601 with its pieces, a chest and a refinery stands as a
// base of the player (character 1), and a second base, 801, of the same player stands on another map. Nothing here is from a real save.

const ddWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,state,serial,owner_account_id,properties) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,5,3,1,jsonb('{"m_SelfUniqueID":"!!act#601"}')),
  (602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5000,6000,7,0,0,0,1,5,4,1,jsonb('{}')),
  (701,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',5010,6000,7,0,0,0.7071,0.7071,5,5,1,jsonb('{}')),
  (702,'/Game/BP_Refinery.BP_Refinery_C','HaggaBasin',5020,6000,7,0,0,0,1,5,6,1,jsonb('{}')),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,90000,0,0,0,0,1,0,7,1,jsonb('{}')),
  (802,'/Game/BP_Building.BP_Building_C','HaggaBasin',90000,90000,0,0,0,0,1,0,8,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values
  (5764607523034234881, jsonb('{"Actor":[[],{"x":1.5}]}')),
  (5764607523034234882, jsonb('{"ContainerInventory":[[],{}]}')),
  (5764607523034234891, jsonb('{"ItemCraftingComponent":[[],{"Queue":[{"ItemUniqueId":"!!itm#9003","InstigatorActorId":"!!act#2","Spare":"!!act#0"}]}]}')),
  (5764607523034235001, jsonb('{"Actor":[[],{}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values
  (601,5764607523034234881,'Actor'),(701,5764607523034234882,'ContainerInventory'),(702,5764607523034234891,'ItemCraftingComponent'),(801,5764607523034235001,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (701,'Ore Out',1,3,1,1),(801,'Other base',1,3,1,1);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (701,1,2),(801,1,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (7001,701,4,20,1000.0),(7002,702,12,5,1000.0),(7801,801,4,20,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values
  (9001,7001,3,0,'CopperOre',0,1790000000,'{}',0,null),
  (9002,7001,5,1,'IronOre',1,1790000001,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.5}]}',2,0.25),
  (9003,7002,1,0,'Scrap',0,1790000002,'{}',0,null),
  (9801,7801,9,0,'CopperOre',0,1790000003,'{}',0,null);
insert into actor_inventories(inventory_id,component_name_hash) values (7001,111),(7002,222),(7801,333);
insert into buildings(id,owner_id) values (602,null),(802,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id) values
  (602,1,'Foundation',5000,6000,7,0,0,0,1,5764607523034234881,0,100.0,0,0,1),
  (602,2,'Wall',5010,6000,7,0,0,0.7071,0.7071,5764607523034234881,0,90.5,1,2,1),
  (602,3,'Roof',5020,6000,10,0,0,0,1,5764607523034234881,0,80.0,1,0,1),
  (802,1,'Foundation',90000,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1),
  (802,2,'Wall',90010,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1);
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (601,5764607523034234881,1000.0,'Totem_Small_Placeable',1,1,0,1),(701,5764607523034234881,50.0,'GenericContainer_Placeable',1,1,0,1),
  (702,5764607523034234881,60.0,'SmallOreRefinery_Placeable',1,1,0,1),(801,5764607523034235001,1000.0,'Totem_Placeable',1,1,0,1);
insert into totems(id,landclaim_vertical_level,last_backup_timestamp,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values
  (601,0,0,5000,6000,7,90),(801,0,0,90000,90000,0,0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,1,0),(801,0,0);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (1,1,'##Totem_Small_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (1,601),(1,602),(1,701),(1,702);`

const ddStanding = `
update actors set state=0 where id in (601,602,701,702);
delete from base_backup_linked_actors; delete from base_backups;
update actors set serial=0 where id=602;
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'Ore Base',1,3,0,1);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(701,5,3),(601,5,2);`
