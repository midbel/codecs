<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="list" select="(/root/item[1], /root/item[2])"/>
		<sequence>
			<xsl:for-each select="$list">
				<item>
					<xsl:value-of select="."/>
				</item>
			</xsl:for-each>
		</sequence>
	</xsl:template>
</xsl:stylesheet>